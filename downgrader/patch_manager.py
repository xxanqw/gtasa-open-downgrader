import os
import posixpath
import shutil
import stat
import tempfile
import zipfile

from icloud_resolver import download_file


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
    nested_root = os.path.join(extracted_dir, "Patches")
    nested_manifest = os.path.join(nested_root, "manifest.json")
    if os.path.isfile(direct_manifest):
        return extracted_dir
    if os.path.isfile(nested_manifest):
        return nested_root
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
