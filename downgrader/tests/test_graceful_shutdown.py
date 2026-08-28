import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

os.environ.setdefault("QT_QPA_PLATFORM", "offscreen")
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from PySide6.QtCore import QThread
from PySide6.QtWidgets import QApplication

import main


class CooperativeWorker(QThread):
    def run(self):
        while not self.isInterruptionRequested():
            self.msleep(1)


class GracefulShutdownTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.application = QApplication.instance() or QApplication([])

    def test_main_window_waits_for_worker_before_closing(self):
        window = main.DowngraderApp()
        worker = CooperativeWorker(window)
        worker.start()
        window.show()

        window.close()

        self.assertTrue(window.shutdown_requested)
        self.assertFalse(worker.isRunning())

    def test_download_dialog_closes_only_after_native_thread_finishes(self):
        with tempfile.TemporaryDirectory() as target_dir:
            with patch("main.patch_manager.download_and_extract_patches"):
                dialog = main.DownloadDialog("https://example.com/Patches.zip", target_dir)
                dialog.exec()

        self.assertTrue(dialog.success)
        self.assertFalse(dialog.thread.isRunning())

    def test_mod_dialog_closes_only_after_native_thread_finishes(self):
        with tempfile.TemporaryDirectory() as game_dir:
            dialog = main.ModInstallDialog(game_dir, [])
            dialog.exec()

        self.assertTrue(dialog.success)
        self.assertFalse(dialog.thread.isRunning())


if __name__ == "__main__":
    unittest.main()
