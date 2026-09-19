import io
import os
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import patch_manager


def zip_archive(files):
    archive_data = io.BytesIO()
    with zipfile.ZipFile(archive_data, "w") as archive:
        for name, contents in files.items():
            archive.writestr(name, contents)
    return archive_data.getvalue()


class PatchDownloadTests(unittest.TestCase):
    def setUp(self):
        self.temporary_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary_dir.cleanup)
        self.target_dir = os.path.join(self.temporary_dir.name, "Patches")

    def _download(self, archive_data):
        def fake_download(url, target_path, progress_callback=None):
            with open(target_path, "wb") as archive_file:
                archive_file.write(archive_data)
            if progress_callback:
                progress_callback(len(archive_data), len(archive_data))
            return target_path

        return fake_download

    def test_uses_package_download_and_installs_valid_archive(self):
        old_file = os.path.join(self.target_dir, "obsolete.xdelta")
        os.makedirs(self.target_dir)
        with open(old_file, "wb") as existing_file:
            existing_file.write(b"old")
        archive_data = zip_archive(
            {
                "manifest.json": b"{}",
                "audio/config.xdelta": b"patch",
            }
        )
        progress = []

        with patch.object(patch_manager, "download_file", side_effect=self._download(archive_data)) as download:
            result = patch_manager.download_and_extract_patches(
                "https://files.xxanqw.me/s/6jhSHGdr8N",
                self.target_dir,
                lambda downloaded, total: progress.append((downloaded, total)),
            )

        self.assertEqual(result, self.target_dir)
        download.assert_called_once()
        self.assertTrue(os.path.isfile(os.path.join(self.target_dir, "manifest.json")))
        self.assertTrue(os.path.isfile(os.path.join(self.target_dir, "audio", "config.xdelta")))
        self.assertFalse(os.path.exists(old_file))
        self.assertEqual(progress, [(len(archive_data), len(archive_data))])

    def test_accepts_archive_with_single_patches_root(self):
        archive_data = zip_archive(
            {
                "Patches/manifest.json": b"{}",
                "Patches/main.xdelta": b"patch",
            }
        )

        with patch.object(patch_manager, "download_file", side_effect=self._download(archive_data)):
            patch_manager.download_and_extract_patches("https://example.invalid", self.target_dir)

        self.assertTrue(os.path.isfile(os.path.join(self.target_dir, "manifest.json")))
        self.assertFalse(os.path.exists(os.path.join(self.target_dir, "Patches")))

    def test_accepts_archive_with_variant_patches_root(self):
        archive_data = zip_archive(
            {
                "Patches_v2/manifest.json": b"{}",
                "Patches_v2/variants/steam_r2_german/anim.xdelta": b"variant_patch",
            }
        )

        with patch.object(patch_manager, "download_file", side_effect=self._download(archive_data)):
            patch_manager.download_and_extract_patches("https://example.invalid", self.target_dir)

        self.assertTrue(os.path.isfile(os.path.join(self.target_dir, "manifest.json")))
        self.assertTrue(os.path.isfile(os.path.join(self.target_dir, "variants", "steam_r2_german", "anim.xdelta")))
        self.assertFalse(os.path.exists(os.path.join(self.target_dir, "Patches_v2")))

    def test_rejects_path_traversal_without_replacing_existing_patches(self):
        os.makedirs(self.target_dir)
        existing_manifest = os.path.join(self.target_dir, "manifest.json")
        with open(existing_manifest, "wb") as manifest_file:
            manifest_file.write(b"existing")
        archive_data = zip_archive(
            {
                "manifest.json": b"replacement",
                "../outside.txt": b"unsafe",
            }
        )

        with patch.object(patch_manager, "download_file", side_effect=self._download(archive_data)):
            with self.assertRaises(ValueError):
                patch_manager.download_and_extract_patches("https://example.invalid", self.target_dir)

        with open(existing_manifest, "rb") as manifest_file:
            self.assertEqual(manifest_file.read(), b"existing")
        self.assertFalse(os.path.exists(os.path.join(self.temporary_dir.name, "outside.txt")))


class DirectDownloadTests(unittest.TestCase):
    def setUp(self):
        self.temporary_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary_dir.cleanup)

    def test_parse_content_disposition_filename(self):
        self.assertEqual(
            patch_manager.parse_content_disposition_filename(
                'attachment; filename="Patches_v2.zip"; filename*=UTF-8\'\'Patches_v2.zip'
            ),
            "Patches_v2.zip"
        )
        self.assertEqual(
            patch_manager.parse_content_disposition_filename('attachment; filename="test_file.zip"'),
            "test_file.zip"
        )
        self.assertEqual(
            patch_manager.parse_content_disposition_filename('inline; filename=unquoted.zip'),
            "unquoted.zip"
        )
        self.assertIsNone(patch_manager.parse_content_disposition_filename(None))
        self.assertIsNone(patch_manager.parse_content_disposition_filename(""))

    def test_download_file_reads_metadata_and_streams(self):
        fake_content = [b"chunk1_", b"chunk2_", b"chunk3"]
        total_bytes = sum(len(c) for c in fake_content)
        
        class FakeResponse:
            def __init__(self):
                self.status_code = 200
                self.headers = {
                    "Content-Length": str(total_bytes),
                    "Content-Disposition": 'attachment; filename="Patches_v2.zip"',
                }
                self.url = "https://files.xxanqw.me/s/6jhSHGdr8N"

            def raise_for_status(self):
                pass

            def iter_content(self, chunk_size=65536):
                for chunk in fake_content:
                    yield chunk

            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc_val, exc_tb):
                pass

        progress_calls = []
        with patch("patch_manager.requests.get", return_value=FakeResponse()) as mock_get:
            out_file = patch_manager.download_file(
                "https://files.xxanqw.me/s/6jhSHGdr8N",
                self.temporary_dir.name,
                progress_callback=lambda d, t: progress_calls.append((d, t)),
            )

        mock_get.assert_called_once()
        self.assertEqual(os.path.basename(out_file), "Patches_v2.zip")
        with open(out_file, "rb") as f:
            self.assertEqual(f.read(), b"".join(fake_content))
        self.assertEqual(progress_calls[-1], (total_bytes, total_bytes))


if __name__ == "__main__":
    unittest.main()

