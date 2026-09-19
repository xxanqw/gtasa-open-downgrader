import os
import posixpath
import re
import shutil
import stat
import tempfile
from urllib.parse import unquote, urlparse
import zipfile

import requests


def parse_content_disposition_filename(header_value):
    """Extract filename from Content-Disposition header if present."""
    if not header_value:
        return None
    # RFC 5987 / RFC 6266: filename*=UTF-8''filename.ext
    match = re.search(r"filename\*\s*=\s*(?:UTF-8|utf-8)''([^;]+)", header_value)
    if match:
        return unquote(match.group(1).strip("\"'"))
    # Standard: filename="filename.ext"
    match = re.search(r'filename\s*=\s*"([^"]+)"', header_value)
    if match:
        return match.group(1)
    # Standard unquoted: filename=filename.ext
    match = re.search(r'filename\s*=\s*([^;]+)', header_value)
    if match:
        return match.group(1).strip("\"' ")
    return None


def download_file(url, destination_path, progress_callback=None, chunk_size=65536, timeout=(15, 60)):
    """
    Downloads a file directly from a URL, following redirects and reading metadata
    (Content-Disposition for filename, Content-Length for size), similar to curl -L -O -J.

    If destination_path is an existing directory, the filename from the server metadata
    (or URL fallback) is appended.
    Returns the absolute path to the downloaded file.
    """
    with requests.get(url, stream=True, allow_redirects=True, timeout=timeout) as response:
        response.raise_for_status()

        content_length = response.headers.get("Content-Length")
        total_size = int(content_length) if content_length and content_length.isdigit() else 0

        cd_header = response.headers.get("Content-Disposition", "")
        filename = parse_content_disposition_filename(cd_header)
        if not filename:
            parsed_path = urlparse(response.url).path
            filename = posixpath.basename(parsed_path) or "download.zip"

        if os.path.isdir(destination_path):
            final_path = os.path.join(destination_path, filename)
        else:
            final_path = destination_path

        final_path = os.path.abspath(final_path)
        os.makedirs(os.path.dirname(final_path), exist_ok=True)

        downloaded = 0
        with open(final_path, "wb") as f:
            for chunk in response.iter_content(chunk_size=chunk_size):
                if chunk:
                    f.write(chunk)
                    downloaded += len(chunk)
                    if progress_callback:
                        progress_callback(downloaded, total_size)

        return final_path


def _safe_extract(archive_path, destination):
    destination = os.path.abspath(destination)
    os.makedirs(destination, exist_ok=True)

    with zipfile.ZipFile(archive_path) as archive:
        for member in archive.infolist():
            member_name = member.filename.replace("\\", "/")
            normalized = posixpath.normpath(member_name).lstrip("/")
            if (
                not normalized
                or normalized == "."
                or normalized == ".."
                or normalized.startswith("../")
                or ":" in normalized.split("/", 1)[0]
            ):
                if normalized in ("", "."):
                    continue
                raise ValueError(f"Unsafe path in patches archive: {member.filename}")

            file_mode = member.external_attr >> 16
            if stat.S_ISLNK(file_mode):
                raise ValueError(f"Symbolic links are not allowed in patches archive: {member.filename}")

            target_path = os.path.abspath(os.path.join(destination, *normalized.split("/")))
            if os.path.commonpath([destination, target_path]) != destination:
                raise ValueError(f"Unsafe extraction target: {member.filename}")

            if member.is_dir():
                os.makedirs(target_path, exist_ok=True)
                continue

            os.makedirs(os.path.dirname(target_path), exist_ok=True)
            with archive.open(member) as source_file, open(target_path, "wb") as target_file:
                shutil.copyfileobj(source_file, target_file)


def _patch_root(extracted_dir):
    direct_manifest = os.path.join(extracted_dir, "manifest.json")
    if os.path.isfile(direct_manifest):
        return extracted_dir
    nested_root = os.path.join(extracted_dir, "Patches")
    if os.path.isfile(os.path.join(nested_root, "manifest.json")):
        return nested_root
    with os.scandir(extracted_dir) as it:
        for entry in it:
            if entry.is_dir():
                candidate = os.path.join(entry.path, "manifest.json")
                if os.path.isfile(candidate):
                    return entry.path
    raise ValueError("The downloaded patches archive does not contain manifest.json.")


def download_and_extract_patches(url, target_dir, progress_callback=None):
    target_dir = os.path.abspath(target_dir)
    parent_dir = os.path.dirname(target_dir)
    os.makedirs(parent_dir, exist_ok=True)

    with tempfile.TemporaryDirectory(prefix=".patch-download-", dir=parent_dir) as work_dir:
        archive_path = os.path.join(work_dir, "Patches.zip")
        extracted_dir = os.path.join(work_dir, "extracted")

        download_file(url, archive_path, progress_callback=progress_callback)
        _safe_extract(archive_path, extracted_dir)
        replacement_dir = _patch_root(extracted_dir)

        backup_dir = os.path.join(work_dir, "previous")
        had_previous = os.path.exists(target_dir)
        if had_previous:
            os.replace(target_dir, backup_dir)

        try:
            os.replace(replacement_dir, target_dir)
        except Exception:
            if had_previous and os.path.exists(backup_dir):
                os.replace(backup_dir, target_dir)
            raise

        if had_previous:
            shutil.rmtree(backup_dir)

    return target_dir
