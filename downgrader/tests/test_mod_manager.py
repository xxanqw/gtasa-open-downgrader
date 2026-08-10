import io
import json
import os
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest.mock import Mock, patch

import py7zr

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import mod_manager


def zip_archive(files):
    archive_data = io.BytesIO()
    with zipfile.ZipFile(archive_data, "w") as archive:
        for name, contents in files.items():
            archive.writestr(name, contents)
    return archive_data.getvalue()


def seven_zip_archive(files):
    archive_data = io.BytesIO()
    with py7zr.SevenZipFile(archive_data, "w") as archive:
        for name, contents in files.items():
            archive.writestr(contents, name)
    return archive_data.getvalue()


class ModTrackerTests(unittest.TestCase):
    def setUp(self):
        self.game_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.game_dir.cleanup)
        os.makedirs(os.path.join(self.game_dir.name, "modloader"))

    def test_legacy_tracker_is_loaded_without_versions(self):
        with open(mod_manager.tracker_path(self.game_dir.name), "w", encoding="utf-8") as tracker_file:
            json.dump({"installed_mods": ["SilentPatch"], "version": "0.1.1"}, tracker_file)

        installed_mods, installed_versions, data = mod_manager.load_mod_tracker(self.game_dir.name)

        self.assertEqual(installed_mods, {"SilentPatch"})
        self.assertEqual(installed_versions, {})
        self.assertEqual(data["version"], "0.1.1")
        self.assertTrue(mod_manager.is_legacy_tracker(data))

    def test_tracker_upgrade_preserves_existing_mods_and_saves_release(self):
        with open(mod_manager.tracker_path(self.game_dir.name), "w", encoding="utf-8") as tracker_file:
            json.dump({"installed_mods": ["ModLoader"], "custom": True}, tracker_file)
        release = {
            "version": "1.1-BUILD34-SA",
            "fingerprint": "github:123",
        }

        mod_manager.update_mod_tracker(
            self.game_dir.name,
            ["SilentPatch"],
            {"SilentPatch": release},
            "v0.1.1",
        )

        installed_mods, installed_versions, data = mod_manager.load_mod_tracker(self.game_dir.name)
        self.assertEqual(installed_mods, {"ModLoader", "SilentPatch"})
        self.assertEqual(installed_versions["SilentPatch"], release)
        self.assertEqual(data["schema_version"], 2)
        self.assertTrue(data["custom"])
        self.assertFalse(mod_manager.is_legacy_tracker(data))


class ReleaseResolutionTests(unittest.TestCase):
    def test_framerate_release_follows_current_official_package_link(self):
        article_response = Mock(
            text=(
                '<a href="https://sharemods.com/newfile123/FramerateVigilante.7z.html">'
                "Download</a>"
            )
        )
        landing_response = Mock(
            text="Uploaded on: <strong>2026-09-01</strong> Size: <strong>250 KB</strong>"
        )
        scraper = Mock()
        scraper.get.side_effect = [article_response, landing_response]

        with patch.object(mod_manager.cloudscraper, "create_scraper", return_value=scraper):
            release = mod_manager.get_mod_release("Framerate Vigilante (60fps fix)")

        self.assertEqual(release["version"], "2026-09-01")
        self.assertEqual(release["file_id"], "newfile123")
        self.assertEqual(
            release["download_url"],
            "https://sharemods.com/newfile123/FramerateVigilante.7z.html",
        )
        self.assertEqual(release["fingerprint"], "sharemods:newfile123:2026-09-01:250 KB")


class ArchiveExtractionTests(unittest.TestCase):
    def setUp(self):
        self.game_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.game_dir.cleanup)

    def test_widescreen_fix_strips_scripts_directory_and_ignores_loader(self):
        archive = zip_archive(
            {
                "scripts/GTASA.WidescreenFix.asi": b"asi",
                "scripts/GTASA.WidescreenFix.ini": b"ini",
                "vorbisFile.dll": b"loader",
            }
        )

        mod_manager.extract_mod_archive(self.game_dir.name, "Widescreen Fixes", archive)

        target = os.path.join(self.game_dir.name, "modloader", "WidescreenFix")
        self.assertTrue(os.path.isfile(os.path.join(target, "GTASA.WidescreenFix.asi")))
        self.assertTrue(os.path.isfile(os.path.join(target, "GTASA.WidescreenFix.ini")))
        self.assertFalse(os.path.exists(os.path.join(target, "scripts")))
        self.assertFalse(os.path.exists(os.path.join(target, "vorbisFile.dll")))

    def test_frontend_archive_preserves_models_layout(self):
        archive = zip_archive({"models/txd/LOADSCS.txd": b"textures"})

        mod_manager.extract_mod_archive(self.game_dir.name, "Frontend Mods", archive)

        installed_file = os.path.join(
            self.game_dir.name,
            "modloader",
            "FrontendMods",
            "models",
            "txd",
            "LOADSCS.txd",
        )
        self.assertTrue(os.path.isfile(installed_file))

    def test_framerate_archive_installs_only_san_andreas_files(self):
        archive = seven_zip_archive(
            {
                "GTA III/FramerateVigilante/FramerateVigilante.III.asi": b"iii",
                "GTA SA/FramerateVigilante/FramerateVigilante.SA.asi": b"sa",
                "GTA SA/FramerateVigilante/FramerateVigilante.ini": b"settings",
                "GTA VC/FramerateVigilante/FramerateVigilante.VC.asi": b"vc",
            }
        )

        mod_manager.extract_mod_archive(
            self.game_dir.name,
            "Framerate Vigilante (60fps fix)",
            archive,
            archive_type="7z",
        )

        target = os.path.join(self.game_dir.name, "modloader", "FramerateVigilante")
        self.assertTrue(os.path.isfile(os.path.join(target, "FramerateVigilante.SA.asi")))
        self.assertTrue(os.path.isfile(os.path.join(target, "FramerateVigilante.ini")))
        self.assertFalse(os.path.exists(os.path.join(target, "FramerateVigilante.III.asi")))
        self.assertFalse(os.path.exists(os.path.join(target, "FramerateVigilante.VC.asi")))

    def test_archive_path_traversal_is_rejected(self):
        archive = zip_archive({"../outside.asi": b"unsafe"})

        with self.assertRaises(ValueError):
            mod_manager.extract_mod_archive(self.game_dir.name, "ModLoader", archive)


if __name__ == "__main__":
    unittest.main()
