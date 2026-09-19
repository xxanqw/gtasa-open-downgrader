import hashlib
import json
import os
import shutil
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch, MagicMock

os.environ.setdefault("QT_QPA_PLATFORM", "offscreen")
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from PySide6.QtWidgets import QApplication
import main

app = QApplication.instance() or QApplication([])

def calc_md5(path):
    h = hashlib.md5()
    with open(path, "rb") as f:
        while chunk := f.read(65536):
            h.update(chunk)
    return h.hexdigest()

class ScannerAndPatchTests(unittest.TestCase):
    def setUp(self):
        self.tmp_dir = tempfile.TemporaryDirectory()
        self.game_dir = os.path.join(self.tmp_dir.name, "game")
        self.patches_dir = os.path.join(self.tmp_dir.name, "patches")
        os.makedirs(self.game_dir)
        os.makedirs(self.patches_dir)

    def tearDown(self):
        self.tmp_dir.cleanup()

    def test_version_detection_german_r2(self):
        exe_path = os.path.join(self.game_dir, "gta-sa.exe")
        with open(exe_path, "wb") as f:
            f.write(b"german_exe_content")
        
        german_hash = calc_md5(exe_path)
        with patch.dict(main.VERSION_HASHES, {german_hash: "NewSteam R2 (German)"}):
            manifest_path = os.path.join(self.patches_dir, "manifest.json")
            with open(manifest_path, "w") as f:
                json.dump({"files": []}, f)

            scanner = main.ScannerThread(self.game_dir, manifest_path)
            results = []
            detected = []
            scanner.completed.connect(lambda res, ver, ro: (results.extend(res), detected.append(ver)))
            scanner.run()

            self.assertEqual(detected, ["NewSteam R2 (German)"])

    def test_scanner_manifest_v2_sources(self):
        # 1. target file (already downgraded)
        f_target = os.path.join(self.game_dir, "target.dat")
        with open(f_target, "wb") as f:
            f.write(b"TARGET_BYTES")
        h_target = calc_md5(f_target)

        # 2. global source file
        f_global = os.path.join(self.game_dir, "global.dat")
        with open(f_global, "wb") as f:
            f.write(b"GLOBAL_BYTES")
        h_global = calc_md5(f_global)

        # 3. german variant file
        f_german = os.path.join(self.game_dir, "german.dat")
        with open(f_german, "wb") as f:
            f.write(b"GERMAN_BYTES")
        h_german = calc_md5(f_german)

        # 4. modified file
        f_mod = os.path.join(self.game_dir, "mod.dat")
        with open(f_mod, "wb") as f:
            f.write(b"MODIFIED_BYTES")

        manifest = {
            "version": "2.0",
            "files": [
                {
                    "path": "target.dat",
                    "action": "patch",
                    "source_hash": "GLOBAL_TARGET_DAT",
                    "target_hash": h_target,
                    "sources": [{"hash": "GLOBAL_TARGET_DAT", "patch": "target.dat.xdelta"}]
                },
                {
                    "path": "global.dat",
                    "action": "patch",
                    "source_hash": h_global,
                    "target_hash": "SOME_TARGET_HASH",
                    "sources": [{"hash": h_global, "patch": "global.dat.xdelta"}]
                },
                {
                    "path": "german.dat",
                    "action": "patch",
                    "source_hash": "GLOBAL_FOR_GERMAN_DAT",
                    "target_hash": "TARGET_FOR_GERMAN",
                    "sources": [
                        {"hash": "GLOBAL_FOR_GERMAN_DAT", "patch": "german.dat.xdelta"},
                        {"hash": h_german, "patch": "variants/steam_r2_german/german.dat.xdelta"}
                    ]
                },
                {
                    "path": "mod.dat",
                    "action": "patch",
                    "source_hash": "ORIGINAL_HASH",
                    "target_hash": "TARGET_HASH",
                    "sources": [{"hash": "ORIGINAL_HASH", "patch": "mod.dat.xdelta"}]
                },
                {
                    "path": "missing.dat",
                    "action": "patch",
                    "source_hash": "SRC",
                    "target_hash": "TGT"
                }
            ]
        }
        manifest_path = os.path.join(self.patches_dir, "manifest.json")
        with open(manifest_path, "w") as f:
            json.dump(manifest, f)

        scanner = main.ScannerThread(self.game_dir, manifest_path)
        scan_results = []
        scanner.completed.connect(lambda res, ver, ro: scan_results.extend(res))
        scanner.run()

        by_path = {r["path"]: r for r in scan_results}
        self.assertEqual(by_path["target.dat"]["status"], "Already Downgraded")
        self.assertEqual(by_path["target.dat"]["needs_patch"], "No")

        self.assertEqual(by_path["global.dat"]["status"], "Original (Needs Patch)")
        self.assertEqual(by_path["global.dat"]["needs_patch"], "Yes")

        self.assertEqual(by_path["german.dat"]["status"], "Original (Needs Patch)")
        self.assertEqual(by_path["german.dat"]["needs_patch"], "Yes")

        self.assertEqual(by_path["mod.dat"]["status"], "Modified")
        self.assertEqual(by_path["mod.dat"]["needs_patch"], "Yes (Force)")

        self.assertEqual(by_path["missing.dat"]["status"], "Missing")
        self.assertEqual(by_path["missing.dat"]["needs_patch"], "N/A")

    def test_patch_thread_variant_selection(self):
        f_german = os.path.join(self.game_dir, "models", "gta3.img")
        os.makedirs(os.path.dirname(f_german), exist_ok=True)
        with open(f_german, "wb") as f:
            f.write(b"GERMAN_GTA3_DATA")
        h_german = calc_md5(f_german)

        variant_patch_rel = "variants/steam_r2_german/models/gta3.img.xdelta"
        variant_patch_full = os.path.join(self.patches_dir, variant_patch_rel)
        os.makedirs(os.path.dirname(variant_patch_full), exist_ok=True)
        with open(variant_patch_full, "wb") as f:
            f.write(b"PATCH_DATA")

        target_bytes = b"TARGET_GTA3_DATA"
        h_target = hashlib.md5(target_bytes).hexdigest()

        manifest = {
            "version": "2.0",
            "files": [
                {
                    "path": "models/gta3.img",
                    "action": "patch",
                    "source_hash": h_target,
                    "target_hash": h_target,
                    "sources": [
                        {"hash": h_german, "patch": variant_patch_rel}
                    ]
                }
            ]
        }

        # Mock subprocess.run for xdelta
        def fake_subprocess_run(cmd, **kwargs):
            # cmd is [xdelta_bin, "-d", "-s", target_file, patch_file, temp_output]
            temp_out = cmd[5]
            with open(temp_out, "wb") as f:
                f.write(target_bytes)
            proc = MagicMock()
            proc.returncode = 0
            return proc

        with patch("subprocess.run", side_effect=fake_subprocess_run):
            thread = main.PatchThread(self.game_dir, manifest, "xdelta3", self.patches_dir)
            completed_result = []
            thread.completed.connect(lambda s, f: completed_result.append((s, f)))
            thread.run()

            self.assertEqual(completed_result, [(1, 0)])
            with open(f_german, "rb") as f:
                self.assertEqual(f.read(), target_bytes)

    def test_patch_thread_verification_failure(self):
        game_file = os.path.join(self.game_dir, "data.bin")
        original_bytes = b"ORIGINAL_DATA"
        with open(game_file, "wb") as f:
            f.write(original_bytes)
        h_original = calc_md5(game_file)

        patch_file = os.path.join(self.patches_dir, "data.bin.xdelta")
        with open(patch_file, "wb") as f:
            f.write(b"PATCH")

        expected_target_hash = "0123456789abcdef0123456789abcdef"

        manifest = {
            "version": "2.0",
            "files": [
                {
                    "path": "data.bin",
                    "action": "patch",
                    "source_hash": h_original,
                    "target_hash": expected_target_hash,
                    "sources": [{"hash": h_original, "patch": "data.bin.xdelta"}]
                }
            ]
        }

        # Mock xdelta producing unexpected output
        def fake_subprocess_run(cmd, **kwargs):
            temp_out = cmd[5]
            with open(temp_out, "wb") as f:
                f.write(b"CORRUPT_OR_WRONG_OUTPUT")
            proc = MagicMock()
            proc.returncode = 0
            return proc

        progress_msgs = []
        with patch("subprocess.run", side_effect=fake_subprocess_run):
            thread = main.PatchThread(self.game_dir, manifest, "xdelta3", self.patches_dir)
            thread.file_progress.connect(lambda i, st, msg: progress_msgs.append((st, msg)))
            completed_result = []
            thread.completed.connect(lambda s, f: completed_result.append((s, f)))
            thread.run()

            self.assertEqual(completed_result, [(0, 1)])
            self.assertTrue(any("Verification failed" in msg for _, msg in progress_msgs))
            # Verify game file was NOT modified/corrupted
            with open(game_file, "rb") as f:
                self.assertEqual(f.read(), original_bytes)
            # Verify temp file was cleaned up
            self.assertFalse(os.path.exists(game_file + ".tmp"))

    def test_patch_thread_copy_action_with_payload(self):
        target_exe_bytes = b"V10_CLEAN_EXE"
        h_target = hashlib.md5(target_exe_bytes).hexdigest()
        
        payload_file = os.path.join(self.patches_dir, "gta_sa.exe")
        with open(payload_file, "wb") as f:
            f.write(target_exe_bytes)

        manifest = {
            "version": "2.0",
            "files": [
                {
                    "path": "gta-sa.exe",
                    "action": "copy",
                    "source_hash": "GLOBAL_HASH",
                    "target_hash": h_target,
                    "payload": "gta_sa.exe",
                    "sources": [
                        {"hash": "GLOBAL_HASH"},
                        {"hash": "GERMAN_HASH"}
                    ]
                }
            ]
        }

        thread = main.PatchThread(self.game_dir, manifest, "xdelta3", self.patches_dir)
        completed_result = []
        thread.completed.connect(lambda s, f: completed_result.append((s, f)))
        thread.run()

        self.assertEqual(completed_result, [(1, 0)])
        # Both gta-sa.exe and gta_sa.exe should be present
        exe1 = os.path.join(self.game_dir, "gta-sa.exe")
        exe2 = os.path.join(self.game_dir, "gta_sa.exe")
        self.assertTrue(os.path.exists(exe1))
        self.assertTrue(os.path.exists(exe2))
        with open(exe1, "rb") as f:
            self.assertEqual(f.read(), target_exe_bytes)
        with open(exe2, "rb") as f:
            self.assertEqual(f.read(), target_exe_bytes)

    def test_version_detection_priority_steam_over_orphan(self):
        german_hash = "d9cb35c898d3298ca904a63e10ee18d7"
        clean_10_hash = "4e99d762f44b1d5e7652dfa7e73d6b6f"

        exe_steam = os.path.join(self.game_dir, "gta-sa.exe")
        exe_orphan = os.path.join(self.game_dir, "gta_sa.exe")
        with open(exe_steam, "w") as f:
            f.write("steam")
        with open(exe_orphan, "w") as f:
            f.write("clean10")

        manifest_path = os.path.join(self.patches_dir, "manifest.json")
        with open(manifest_path, "w") as f:
            json.dump({"files": []}, f)

        with patch("main.calculate_md5", side_effect=lambda p, *args: german_hash if "gta-sa.exe" in p else clean_10_hash):
            scanner = main.ScannerThread(self.game_dir, manifest_path)
            detected = []
            scanner.completed.connect(lambda res, ver, ro: detected.append(ver))
            scanner.run()

            self.assertEqual(detected, ["NewSteam R2 (German)"])

if __name__ == "__main__":
    unittest.main()
