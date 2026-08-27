import html
import io
import json
import os
import posixpath
import re
import shutil
import tempfile
import zipfile
from datetime import datetime, timezone
from urllib.parse import urlsplit

import cloudscraper
import py7zr
import requests


TRACKER_SCHEMA_VERSION = 2
GITHUB_API = "https://api.github.com/repos"
REQUEST_HEADERS = {
    "Accept": "application/vnd.github+json",
    "User-Agent": "gtasa-open-downgrader",
}

GITHUB_RELEASES = {
    "ModLoader": {
        "repository": "thelink2012/modloader",
        "endpoint": "releases/latest",
        "asset": "modloader.zip",
    },
    "SilentPatch": {
        "repository": "CookiePLMonster/SilentPatch",
        "endpoint": "releases/latest",
        "asset": "SilentPatchSA.zip",
    },
    "Widescreen Fixes": {
        "repository": "ThirteenAG/WidescreenFixesPack",
        "endpoint": "releases/tags/gtasa",
        "asset": "GTASA.WidescreenFix.zip",
        "date_version": True,
    },
    "SkyGFX": {
        "repository": "aap/skygfx",
        "endpoint": "releases/latest",
        "asset_prefix": "skygfx_sa_",
    },
    "Frontend Mods": {
        "repository": "ThirteenAG/WidescreenFixesPack",
        "endpoint": "releases/tags/gtasa",
        "asset": "GTASA.WidescreenFrontend.zip",
        "date_version": True,
    },
    "Project 2DFX": {
        "repository": "ThirteenAG/III.VC.SA.IV.Project2DFX",
        "endpoint": "releases/tags/gtasa",
        "asset": "SA.Project2DFX.zip",
        "date_version": True,
    },
}

STATIC_RELEASES = {
    "ASI Loader": {
        "version": "1.3",
        "url": "https://silent.rockstarvision.com/uploads/silents_asi_loader_13.zip",
        "source_url": "https://silentsblog.com/mods/gta-sa/#asiloader",
    },
    "Framerate Vigilante (60fps fix)": {
        "fallback_version": "2026-08-16",
        "fallback_url": "https://sharemods.com/0cxm88ppiv38/Framerate_Vigilante.zip.html",
        "fallback_file_id": "0cxm88ppiv38",
        "archive_type": "zip",
        "sharemods": True,
        "source_url": "https://www.mixmods.com.br/2022/08/iii-vc-sa-framerate-vigilante/",
    },
    "GInput": {
        "version": "2016-02-09",
        "url": "https://silent.rockstarvision.com/uploads/GInputSA.zip",
        "source_url": "https://silentsblog.com/mods/gta-sa/#ginput",
    },
}

SUPPORTED_MODS = frozenset(GITHUB_RELEASES) | frozenset(STATIC_RELEASES)


def _sharemods_scraper():
    # ShareMods currently challenges cloudscraper's default Chrome/Opera
    # profiles, while its Firefox profile is accepted on both Windows and Linux.
    return cloudscraper.create_scraper(
        browser={"browser": "firefox", "platform": "windows", "desktop": True}
    )


def safe_request(method, url, **kwargs):
    kwargs.setdefault("headers", REQUEST_HEADERS)
    try:
        return requests.request(method, url, verify=True, **kwargs)
    except requests.exceptions.SSLError:
        import urllib3

        urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
        return requests.request(method, url, verify=False, **kwargs)


def _find_release_asset(release, source):
    expected_name = source.get("asset")
    expected_prefix = source.get("asset_prefix", "")
    for asset in release.get("assets", []):
        asset_name = asset.get("name", "")
        if expected_name and asset_name == expected_name:
            return asset
        if expected_prefix and asset_name.lower().startswith(expected_prefix) and asset_name.lower().endswith(".zip"):
            return asset
    return None


def _github_release(mod_name, cache):
    source = GITHUB_RELEASES[mod_name]
    api_url = f"{GITHUB_API}/{source['repository']}/{source['endpoint']}"
    if api_url not in cache:
        response = safe_request("GET", api_url, timeout=20)
        response.raise_for_status()
        cache[api_url] = response.json()

    release = cache[api_url]
    asset = _find_release_asset(release, source)
    if not asset:
        expected = source.get("asset") or source.get("asset_prefix")
        raise RuntimeError(f"Could not find {expected} in the latest {mod_name} release.")

    updated_at = asset.get("updated_at", "")
    if source.get("date_version"):
        version = updated_at[:10] or release.get("tag_name", "unknown")
    else:
        version = release.get("tag_name") or updated_at[:10] or "unknown"

    return {
        "version": version,
        "fingerprint": f"github:{asset.get('id')}:{updated_at}:{asset.get('size')}",
        "download_url": asset["browser_download_url"],
        "source_url": release.get("html_url", f"https://github.com/{source['repository']}/releases"),
    }


def _static_release(mod_name):
    source = STATIC_RELEASES[mod_name]
    if source.get("sharemods"):
        scraper = _sharemods_scraper()
        landing_url = source["fallback_url"]
        file_id = source["fallback_file_id"]
        archive_type = source["archive_type"]
        try:
            article_response = scraper.get(source["source_url"], timeout=20)
            article_response.raise_for_status()
            sharemods_match = re.search(
                r'https?://(?:www\.)?sharemods\.com/(?P<file_id>[^/"\'<>\s]+)/'
                r'(?P<filename>[^"\'<>\s]+\.(?P<archive_type>zip|7z)\.html)',
                article_response.text,
                re.IGNORECASE,
            )
            if sharemods_match:
                file_id = sharemods_match.group("file_id")
                landing_url = html.unescape(sharemods_match.group(0))
                archive_type = sharemods_match.group("archive_type").lower()
        except requests.RequestException:
            pass

        landing_html = ""
        try:
            response = scraper.get(landing_url, timeout=20)
            response.raise_for_status()
            landing_html = response.text
        except requests.RequestException:
            # ShareMods may challenge automated metadata requests. The package
            # URL found on the official article is still the authoritative one.
            pass

        uploaded_match = re.search(r"Uploaded on:\s*<strong>([^<]+)", landing_html, re.IGNORECASE)
        size_match = re.search(r"Size:\s*<strong>([^<]+)", landing_html, re.IGNORECASE)
        uploaded = uploaded_match.group(1).strip() if uploaded_match else "unknown"
        size = size_match.group(1).strip() if size_match else "unknown"
        return {
            "version": uploaded if uploaded != "unknown" else source["fallback_version"],
            "fingerprint": f"sharemods:{file_id}:{uploaded}:{size}",
            "download_url": landing_url,
            "source_url": source["source_url"],
            "file_id": file_id,
            "archive_type": archive_type,
        }

    fingerprint_parts = ["static", source["url"], source["version"]]
    try:
        response = safe_request("HEAD", source["url"], allow_redirects=True, timeout=15)
        response.raise_for_status()
        fingerprint_parts.extend(
            [
                response.headers.get("ETag", ""),
                response.headers.get("Last-Modified", ""),
                response.headers.get("Content-Length", ""),
            ]
        )
    except requests.RequestException:
        pass

    return {
        "version": source["version"],
        "fingerprint": ":".join(fingerprint_parts),
        "download_url": source["url"],
        "source_url": source["source_url"],
    }


def get_mod_release(mod_name, cache=None):
    cache = cache if cache is not None else {}
    if mod_name in GITHUB_RELEASES:
        return _github_release(mod_name, cache)
    if mod_name in STATIC_RELEASES:
        return _static_release(mod_name)
    raise ValueError(f"Unknown mod: {mod_name}")


def _normalized_member_name(name):
    normalized = posixpath.normpath(name.replace("\\", "/")).lstrip("/")
    if normalized in ("", "."):
        return ""
    if normalized == ".." or normalized.startswith("../"):
        raise ValueError(f"Unsafe path in mod archive: {name}")
    return normalized


def _extract_zip(archive_data, target_dir, map_member):
    target_dir = os.path.abspath(target_dir)
    os.makedirs(target_dir, exist_ok=True)

    with zipfile.ZipFile(io.BytesIO(archive_data)) as archive:
        for member in archive.infolist():
            normalized = _normalized_member_name(member.filename)
            relative_path = map_member(normalized)
            if not relative_path:
                continue

            relative_path = _normalized_member_name(relative_path)
            destination = os.path.abspath(os.path.join(target_dir, *relative_path.split("/")))
            if os.path.commonpath([target_dir, destination]) != target_dir:
                raise ValueError(f"Unsafe extraction target: {member.filename}")

            if member.is_dir():
                os.makedirs(destination, exist_ok=True)
                continue

            os.makedirs(os.path.dirname(destination), exist_ok=True)
            with archive.open(member) as source_file, open(destination, "wb") as target_file:
                target_file.write(source_file.read())


def _extract_7z(archive_data, target_dir, map_member):
    target_dir = os.path.abspath(target_dir)
    os.makedirs(target_dir, exist_ok=True)

    with tempfile.TemporaryDirectory() as temporary_dir:
        with py7zr.SevenZipFile(io.BytesIO(archive_data), mode="r") as archive:
            mapped_files = []
            for member in archive.list():
                if member.is_directory:
                    continue
                member_name = member.filename
                normalized = _normalized_member_name(member_name)
                relative_path = map_member(normalized)
                if not relative_path:
                    continue
                relative_path = _normalized_member_name(relative_path)
                mapped_files.append((member_name, relative_path))
            archive.extract(path=temporary_dir, targets=[name for name, _ in mapped_files])

        for member_name, relative_path in mapped_files:
            source_path = os.path.join(temporary_dir, *member_name.replace("\\", "/").split("/"))
            destination = os.path.abspath(os.path.join(target_dir, *relative_path.split("/")))
            if os.path.commonpath([target_dir, destination]) != target_dir:
                raise ValueError(f"Unsafe extraction target: {member_name}")
            os.makedirs(os.path.dirname(destination), exist_ok=True)
            shutil.copyfile(source_path, destination)


def _identity(name):
    return name


def _strip_prefix(prefix):
    def mapper(name):
        if name == prefix.rstrip("/"):
            return None
        if name.startswith(prefix):
            return name[len(prefix):]
        return None

    return mapper


def _strip_first_directory(name):
    if "/" not in name:
        return None
    return name.split("/", 1)[1]


def _framerate_member(name):
    path_parts = name.split("/")
    if "gta sa" not in (part.lower() for part in path_parts[:-1]):
        return None
    filename = posixpath.basename(name)
    if filename.lower() in ("frameratevigilante.sa.asi", "frameratevigilante.ini"):
        return filename
    return None


def _ginput_member(name):
    if name.startswith("GInputAPI (for modders)/"):
        return None
    return name


def _silent_patch_member(name):
    if name.lower().endswith((".asi", ".ini")):
        return posixpath.basename(name)
    return None


def _asi_loader_member(name):
    if name in ("vorbisFile.dll", "vorbisHooked.dll") or name.startswith("scripts/"):
        return name
    return None


def extract_mod_archive(game_path, mod_name, archive_data, archive_type="zip"):
    modloader_dir = os.path.join(game_path, "modloader")
    extractors = {
        "ASI Loader": (game_path, _asi_loader_member),
        "ModLoader": (game_path, _identity),
        "SilentPatch": (os.path.join(modloader_dir, "SilentPatch"), _silent_patch_member),
        "Widescreen Fixes": (os.path.join(modloader_dir, "WidescreenFix"), _strip_prefix("scripts/")),
        "SkyGFX": (os.path.join(modloader_dir, "SkyGFX"), _strip_first_directory),
        "Frontend Mods": (os.path.join(modloader_dir, "FrontendMods"), _identity),
        "Framerate Vigilante (60fps fix)": (
            os.path.join(modloader_dir, "FramerateVigilante"),
            _framerate_member,
        ),
        "GInput": (os.path.join(modloader_dir, "GInput"), _ginput_member),
        "Project 2DFX": (os.path.join(modloader_dir, "Project2DFX"), _identity),
    }
    if mod_name not in extractors:
        raise ValueError(f"Unknown mod: {mod_name}")
    target_dir, map_member = extractors[mod_name]
    if archive_type == "7z":
        _extract_7z(archive_data, target_dir, map_member)
    else:
        _extract_zip(archive_data, target_dir, map_member)


def _sharemods_download_url(landing_url, file_id):
    scraper = _sharemods_scraper()
    landing_response = scraper.get(landing_url, timeout=20)
    landing_response.raise_for_status()
    download_response = scraper.post(
        landing_url,
        data={
            "op": "download2",
            "id": file_id,
            "rand": "",
            "referer": "",
            "method_free": "",
            "method_premium": "",
        },
        timeout=20,
    )
    download_response.raise_for_status()
    for raw_url in re.findall(r'href=["\'](https://[^"\']+)["\']', download_response.text):
        candidate = html.unescape(raw_url)
        parsed = urlsplit(candidate)
        hostname = (parsed.hostname or "").lower()
        is_r2_download = hostname == "r2.cloudflarestorage.com" or hostname.endswith(
            ".r2.cloudflarestorage.com"
        )
        is_ovh_download = (
            hostname == "sharem.s3.de.io.cloud.ovh.net"
            and parsed.path.startswith("/uploads/")
            and parsed.path.rstrip("/").endswith(f"/{file_id}")
        )
        if parsed.scheme == "https" and (is_r2_download or is_ovh_download):
            return candidate

    raise RuntimeError("Could not create the official Framerate Vigilante download link.")


def install_mod(game_path, mod_name):
    release = get_mod_release(mod_name)
    download_url = release["download_url"]
    if release.get("file_id"):
        download_url = _sharemods_download_url(download_url, release["file_id"])
    response = safe_request("GET", download_url, timeout=90)
    response.raise_for_status()
    extract_mod_archive(game_path, mod_name, response.content, release.get("archive_type", "zip"))
    return {
        "version": release["version"],
        "fingerprint": release["fingerprint"],
        "source_url": release["source_url"],
        "installed_at": datetime.now(timezone.utc).isoformat(),
    }


def tracker_path(game_path):
    return os.path.join(game_path, "modloader", ".downgrader")


def load_mod_tracker(game_path):
    path = tracker_path(game_path)
    data = {}
    if os.path.exists(path):
        try:
            with open(path, "r", encoding="utf-8") as tracker_file:
                loaded = json.load(tracker_file)
                if isinstance(loaded, dict):
                    data = loaded
        except (OSError, ValueError):
            pass

    installed_mods = data.get("installed_mods", [])
    if not isinstance(installed_mods, list):
        installed_mods = []
    installed_versions = data.get("mod_versions", {})
    if not isinstance(installed_versions, dict):
        installed_versions = {}
    return set(installed_mods), installed_versions, data


def is_legacy_tracker(data):
    if not data:
        return False
    schema_version = data.get("schema_version")
    return not isinstance(schema_version, int) or schema_version < TRACKER_SCHEMA_VERSION


def expand_legacy_mod_selection(selected_mods, installed_mods, tracker_data):
    expanded = list(dict.fromkeys(selected_mods))
    if is_legacy_tracker(tracker_data):
        for mod_name in sorted(set(installed_mods) & SUPPORTED_MODS):
            if mod_name not in expanded:
                expanded.append(mod_name)
    return expanded


def update_mod_tracker(game_path, installed_mods, installed_releases, app_version):
    modloader_dir = os.path.join(game_path, "modloader")
    os.makedirs(modloader_dir, exist_ok=True)

    existing_mods, existing_versions, data = load_mod_tracker(game_path)
    existing_mods.update(installed_mods)
    existing_versions.update(installed_releases)
    data.update(
        {
            "schema_version": TRACKER_SCHEMA_VERSION,
            "installed_mods": sorted(existing_mods),
            "mod_versions": existing_versions,
            "version": app_version,
        }
    )

    path = tracker_path(game_path)
    temporary_path = f"{path}.tmp"
    with open(temporary_path, "w", encoding="utf-8") as tracker_file:
        json.dump(data, tracker_file, indent=2, sort_keys=True)
    os.replace(temporary_path, path)
