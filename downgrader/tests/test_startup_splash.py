import builtins
import os
import sys
import unittest
from pathlib import Path
from unittest.mock import patch

os.environ.setdefault("QT_QPA_PLATFORM", "offscreen")
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from PySide6.QtWidgets import QApplication

import main


class StartupSplashTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.application = QApplication.instance() or QApplication([])

    def test_permission_error_from_packaged_splash_falls_back_to_qt(self):
        original_import = builtins.__import__

        def guarded_import(name, *args, **kwargs):
            if name == "pyi_splash":
                raise PermissionError("local sockets are unavailable")
            return original_import(name, *args, **kwargs)

        with patch("builtins.__import__", side_effect=guarded_import):
            startup_splash = main.StartupSplash(self.application)

        self.assertIsNotNone(startup_splash.qt_splash)
        startup_splash.close()


if __name__ == "__main__":
    unittest.main()
