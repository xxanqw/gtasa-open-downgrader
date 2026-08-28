import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from main import ModInstallThread
import mod_manager


class ModInstallThreadTests(unittest.TestCase):
    def create_tracked_game(self, installed_mods, legacy=False):
        temp_dir = tempfile.TemporaryDirectory()
        modloader_dir = os.path.join(temp_dir.name, "modloader")
        os.makedirs(modloader_dir)
        tracker = {"installed_mods": installed_mods}
        if not legacy:
            tracker.update(
                {
                    "schema_version": mod_manager.TRACKER_SCHEMA_VERSION,
                    "mod_versions": {
                        mod_name: {"version": "installed", "fingerprint": f"installed:{mod_name}"}
                        for mod_name in installed_mods
                    },
                }
            )
        with open(os.path.join(modloader_dir, ".downgrader"), "w") as config_file:
            json.dump(tracker, config_file)
        return temp_dir

    def test_tracked_mod_is_skipped_by_default(self):
        game_dir = self.create_tracked_game(["SilentPatch"])
        self.addCleanup(game_dir.cleanup)
        thread = ModInstallThread(game_dir.name, ["SilentPatch"])
        thread.apply_mod = Mock()
        results = []
        thread.completed.connect(lambda success, error: results.append((success, error)))

        thread.run()

        thread.apply_mod.assert_not_called()
        self.assertEqual(results, [(True, "")])

    def test_tracked_mod_is_installed_when_force_reinstalling(self):
        game_dir = self.create_tracked_game(["SilentPatch"])
        self.addCleanup(game_dir.cleanup)
        thread = ModInstallThread(
            game_dir.name,
            ["SilentPatch"],
            force_reinstall=True,
        )
        installed_release = {
            "version": "1.1-BUILD34-SA",
            "fingerprint": "github:123",
            "source_url": "https://example.com/release",
            "installed_at": "2026-08-10T00:00:00+00:00",
        }
        thread.apply_mod = Mock(return_value=installed_release)
        results = []
        thread.completed.connect(lambda success, error: results.append((success, error)))

        thread.run()

        thread.apply_mod.assert_called_once_with("SilentPatch")
        self.assertEqual(results, [(True, "")])
        _, installed_versions, _ = mod_manager.load_mod_tracker(game_dir.name)
        self.assertEqual(installed_versions["SilentPatch"], installed_release)

    def test_legacy_tracker_reinstalls_every_recorded_mod(self):
        game_dir = self.create_tracked_game(["SilentPatch", "GInput"], legacy=True)
        self.addCleanup(game_dir.cleanup)
        thread = ModInstallThread(game_dir.name, ["SilentPatch"])
        installed_release = {
            "version": "current",
            "fingerprint": "current-package",
        }
        thread.apply_mod = Mock(return_value=installed_release)
        results = []
        thread.completed.connect(lambda success, error: results.append((success, error)))

        thread.run()

        self.assertEqual(
            [call.args[0] for call in thread.apply_mod.call_args_list],
            ["SilentPatch", "GInput"],
        )
        self.assertEqual(results, [(True, "")])
        installed_mods, installed_versions, data = mod_manager.load_mod_tracker(game_dir.name)
        self.assertEqual(installed_mods, {"SilentPatch", "GInput"})
        self.assertEqual(set(installed_versions), {"SilentPatch", "GInput"})
        self.assertEqual(data["schema_version"], mod_manager.TRACKER_SCHEMA_VERSION)


if __name__ == "__main__":
    unittest.main()
