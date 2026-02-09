import sys
import os
import json
import hashlib
import requests
import zipfile
import io
import time
from PySide6.QtWidgets import (
    QApplication, QMainWindow, QWidget, QVBoxLayout, QHBoxLayout,
    QLabel, QLineEdit, QPushButton, QGroupBox, QCheckBox,
    QTableWidget, QTableWidgetItem, QHeaderView, QStatusBar,
    QFileDialog, QGridLayout, QMessageBox, QDialog, QProgressBar
)
from PySide6.QtGui import QIcon
from PySide6.QtCore import Qt, QThread, Signal
import linux_tools
import icloud_resolver
import updater

class DownloadThread(QThread):
    progress = Signal(int, int, float, float)
    finished = Signal(bool)

    def __init__(self, url, target_dir):
        super().__init__()
        self.url = url
        self.target_dir = target_dir
        self.start_time = 0

    def run(self):
        self.start_time = time.time()
        
        def callback(downloaded, total):
            elapsed = time.time() - self.start_time
            speed = downloaded / elapsed if elapsed > 0 else 0
            time_left = (total - downloaded) / speed if speed > 0 else 0
            self.progress.emit(downloaded, total, speed, time_left)

        success = icloud_resolver.download_and_extract_patches(self.url, self.target_dir, callback)
        self.finished.emit(success)

class DownloadDialog(QDialog):
    def __init__(self, url, target_dir):
        super().__init__()
        self.setWindowTitle("Downloading Patches")
        self.setFixedSize(400, 150)
        self.setWindowFlags(Qt.Window | Qt.WindowTitleHint | Qt.CustomizeWindowHint)
        
        layout = QVBoxLayout(self)
        
        self.label = QLabel("Initializing download...")
        layout.addWidget(self.label)
        
        self.progress_bar = QProgressBar()
        layout.addWidget(self.progress_bar)
        
        self.details_label = QLabel("0 MB / 0 MB (0 KB/s) - --:-- left")
        self.details_label.setStyleSheet("font-size: 10px; color: #666;")
        layout.addWidget(self.details_label)
        
        self.thread = DownloadThread(url, target_dir)
        self.thread.progress.connect(self.update_progress)
        self.thread.finished.connect(self.on_finished)
        self.thread.start()
        
        self.success = False

    def update_progress(self, downloaded, total, speed, time_left):
        if total > 0:
            percent = int((downloaded / total) * 100)
            self.progress_bar.setValue(percent)
            
            d_mb = downloaded / 1048576
            t_mb = total / 1048576
            s_kb = speed / 1024
            
            m, s = divmod(int(time_left), 60)
            time_str = f"{m:02d}:{s:02d}"
            
            self.label.setText("Downloading patch assets...")
            self.details_label.setText(f"{d_mb:.1f} MB / {t_mb:.1f} MB ({s_kb:.1f} KB/s) - {time_str} left")

    def on_finished(self, success):
        self.success = success
        self.accept()

class ModInstallThread(QThread):
    progress = Signal(int, str)
    finished = Signal(bool, str)

    def __init__(self, game_path, selected_mods):
        super().__init__()
        self.game_path = game_path
        self.selected_mods = selected_mods

    def run(self):
        try:
            priority_mods = ["ASI Loader", "ModLoader"]
            to_install = [m for m in priority_mods if m in self.selected_mods]
            to_install += [m for m in self.selected_mods if m not in priority_mods]

            installed_already = set()
            modloader_path = os.path.join(self.game_path, "modloader")
            config_path = os.path.join(modloader_path, ".downgrader")
            if os.path.exists(config_path):
                try:
                    with open(config_path, 'r') as f:
                        data = json.load(f)
                        installed_already = set(data.get("installed_mods", []))
                except Exception:
                    pass

            for i, mod_name in enumerate(to_install):
                self.progress.emit(i + 1, mod_name)
                if mod_name in installed_already:
                    continue
                
                self.apply_mod(mod_name)
            
            if os.path.exists(modloader_path):
                try:
                    installed = set()
                    if os.path.exists(config_path):
                        with open(config_path, 'r') as f:
                            data = json.load(f)
                            installed.update(data.get("installed_mods", []))
                    
                    installed.update(to_install)
                    with open(config_path, 'w') as f:
                        json.dump({"installed_mods": list(installed), "version": "0.1.1"}, f)
                except Exception:
                    pass

            self.finished.emit(True, "")
        except Exception as e:
            self.finished.emit(False, str(e))

    def apply_mod(self, mod_name):
        def safe_get(url, **kwargs):
            try:
                return requests.get(url, verify=True, **kwargs)
            except requests.exceptions.SSLError:
                import urllib3
                urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
                return requests.get(url, verify=False, **kwargs)

        if mod_name == "ASI Loader":
            url = "https://silent.rockstarvision.com/uploads/silents_asi_loader_13.zip"
            response = safe_get(url, timeout=30)
            response.raise_for_status()
            with zipfile.ZipFile(io.BytesIO(response.content)) as z:
                to_extract = ["vorbisFile.dll", "vorbisHooked.dll"]
                for file_name in to_extract:
                    if file_name in z.namelist():
                        with open(os.path.join(self.game_path, file_name), "wb") as f:
                            f.write(z.read(file_name))
                for member in z.namelist():
                    if member.startswith("scripts/"):
                        z.extract(member, self.game_path)
        
        elif mod_name == "ModLoader":
            url = "https://fs.xserv.pp.ua/files/modloader.zip"
            response = safe_get(url, timeout=30)
            response.raise_for_status()
            with zipfile.ZipFile(io.BytesIO(response.content)) as z:
                z.extractall(self.game_path)

        elif mod_name == "SilentPatch":
            api_url = "https://api.github.com/repos/CookiePLMonster/SilentPatch/releases/latest"
            response = safe_get(api_url, timeout=15)
            response.raise_for_status()
            assets = response.json().get("assets", [])
            download_url = next((a["browser_download_url"] for a in assets if a["name"] == "SilentPatchSA.zip"), None)
            
            if download_url:
                response = safe_get(download_url, timeout=30)
                response.raise_for_status()
                target_dir = os.path.join(self.game_path, "modloader", "SilentPatch")
                os.makedirs(target_dir, exist_ok=True)
                with zipfile.ZipFile(io.BytesIO(response.content)) as z:
                    for member in z.namelist():
                        if member.lower().endswith((".asi", ".ini")):
                            filename = os.path.basename(member)
                            if filename:
                                with open(os.path.join(target_dir, filename), "wb") as f:
                                    f.write(z.read(member))
            else:
                raise Exception("Could not find SilentPatchSA.zip in latest GitHub release.")

        elif mod_name == "Widescreen Fixes":
            fixes = [
                ("https://github.com/ThirteenAG/WidescreenFixesPack/releases/download/gtasa/GTASA.WidescreenFix.zip", "WidescreenFix"),
                ("https://github.com/ThirteenAG/WidescreenFixesPack/releases/download/gtasa/GTASA.WidescreenFrontend.zip", "WidescreenFrontend")
            ]
            for url, folder_name in fixes:
                response = safe_get(url, timeout=60)
                response.raise_for_status()
                target_dir = os.path.join(self.game_path, "modloader", folder_name)
                os.makedirs(target_dir, exist_ok=True)
                with zipfile.ZipFile(io.BytesIO(response.content)) as z:
                    z.extractall(target_dir)

        elif mod_name == "SkyGFX":
            api_url = "https://api.github.com/repos/aap/skygfx/releases/latest"
            response = safe_get(api_url, timeout=15)
            response.raise_for_status()
            assets = response.json().get("assets", [])
            download_url = next((a["browser_download_url"] for a in assets if "sa" in a["name"].lower() and a["name"].endswith(".zip")), None)
            
            if not download_url and assets:
                download_url = next((a["browser_download_url"] for a in assets if a["name"].endswith(".zip")), None)

            if download_url:
                response = safe_get(download_url, timeout=60)
                response.raise_for_status()
                target_dir = os.path.join(self.game_path, "modloader", "SkyGFX")
                os.makedirs(target_dir, exist_ok=True)
                with zipfile.ZipFile(io.BytesIO(response.content)) as z:
                    z.extractall(target_dir)
            else:
                raise Exception("Could not find SkyGFX zip in latest GitHub release.")

        elif mod_name == "Frontend Mods":
            url = "https://fs.xserv.pp.ua/files/Frontend%20Mods.zip"
            response = safe_get(url, timeout=60)
            response.raise_for_status()
            target_dir = os.path.join(self.game_path, "modloader", "FrontendMods")
            os.makedirs(target_dir, exist_ok=True)
            with zipfile.ZipFile(io.BytesIO(response.content)) as z:
                z.extractall(target_dir)

        elif mod_name == "Framerate Vigilante (60fps fix)":
            url = "https://fs.xserv.pp.ua/files/Framerate%20Vigilante.zip"
            response = safe_get(url, timeout=60)
            response.raise_for_status()
            target_dir = os.path.join(self.game_path, "modloader", "FramerateVigilante")
            os.makedirs(target_dir, exist_ok=True)
            with zipfile.ZipFile(io.BytesIO(response.content)) as z:
                z.extractall(target_dir)

        elif mod_name == "GInput":
            url = "https://silent.rockstarvision.com/uploads/GInputSA.zip"
            response = safe_get(url, timeout=30)
            response.raise_for_status()
            target_dir = os.path.join(self.game_path, "modloader", "GInput")
            os.makedirs(target_dir, exist_ok=True)
            with zipfile.ZipFile(io.BytesIO(response.content)) as z:
                for member in z.namelist():
                    if "GInputAPI (for modders)" not in member:
                        z.extract(member, target_dir)

        elif mod_name == "Project 2DFX":
            api_url = "https://api.github.com/repos/ThirteenAG/III.VC.SA.IV.Project2DFX/releases/tags/gtasa"
            response = safe_get(api_url, timeout=15)
            response.raise_for_status()
            assets = response.json().get("assets", [])
            download_url = next((a["browser_download_url"] for a in assets if "gtasa" in a["name"].lower() and a["name"].endswith(".zip")), None)
            
            if not download_url and assets:
                download_url = next((a["browser_download_url"] for a in assets if a["name"].endswith(".zip")), None)

            if download_url:
                response = safe_get(download_url, timeout=60)
                response.raise_for_status()
                target_dir = os.path.join(self.game_path, "modloader", "Project2DFX")
                os.makedirs(target_dir, exist_ok=True)
                with zipfile.ZipFile(io.BytesIO(response.content)) as z:
                    z.extractall(target_dir)
            else:
                raise Exception("Could not find Project 2DFX zip for GTA SA in latest GitHub release.")

class ModInstallDialog(QDialog):
    def __init__(self, game_path, selected_mods):
        super().__init__()
        self.setWindowTitle("Installing Mods")
        self.setFixedSize(400, 120)
        self.setWindowFlags(Qt.Window | Qt.WindowTitleHint | Qt.CustomizeWindowHint)
        
        layout = QVBoxLayout(self)
        self.label = QLabel("Initializing mod installation...")
        layout.addWidget(self.label)
        
        self.progress_bar = QProgressBar()
        self.progress_bar.setMaximum(len(selected_mods))
        layout.addWidget(self.progress_bar)
        
        self.thread = ModInstallThread(game_path, selected_mods)
        self.thread.progress.connect(self.update_progress)
        self.thread.finished.connect(self.on_finished)
        self.thread.start()
        
        self.success = False
        self.error_message = ""

    def update_progress(self, current, mod_name):
        self.progress_bar.setValue(current)
        self.label.setText(f"Installing {mod_name}...")

    def on_finished(self, success, error_message):
        self.success = success
        self.error_message = error_message
        self.accept()

class LinuxLaunchOptionsDialog(QDialog):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("Linux Launch Options")
        self.setFixedSize(450, 160)
        
        layout = QVBoxLayout(self)
        layout.setSpacing(5)
        layout.setContentsMargins(10, 10, 10, 10)
        
        layout.addWidget(QLabel("<b>ASI Loader detected on Linux</b>"))
        layout.addWidget(QLabel("Add this to your Steam Game Launch Options:"))
        
        self.command_edit = QLineEdit()
        self.command_edit.setText('WINEDLLOVERRIDES="vorbisFile=n,b" %command%')
        self.command_edit.setReadOnly(True)
        layout.addWidget(self.command_edit)
        
        btn_layout = QHBoxLayout()
        copy_btn = QPushButton("Copy to Clipboard")
        copy_btn.clicked.connect(self.copy_to_clipboard)
        ok_btn = QPushButton("OK")
        ok_btn.clicked.connect(self.accept)
        btn_layout.addWidget(copy_btn)
        btn_layout.addWidget(ok_btn)
        layout.addLayout(btn_layout)

    def copy_to_clipboard(self):
        QApplication.clipboard().setText(self.command_edit.text())

class AboutDialog(QDialog):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("About GTA SA Open Downgrader")
        self.setFixedSize(500, 500)
        
        layout = QVBoxLayout(self)
        
        from PySide6.QtWidgets import QTextEdit, QTabWidget
        
        credits_label = QLabel(
            "<b>GTA SA Open Downgrader</b><br><br>"
            "This project is an open-source replication of the original RockstarNexus Downgrader.<br>"
            "Special thanks to the previous <b>RockstarNexus</b> developers for creating the initial application.<br><br>"
            "Developer: <b>Ivan Potiienko (xxanqw)</b><br>"
            "<a href='https://github.com/xxanqw'>https://github.com/xxanqw</a><br><br>"
            "Thanks to all of the <b>GTA SA community</b> for their continuous support and work!<br><br>"
            "This application is licensed under the <b>MIT License</b>."
        )
        credits_label.setOpenExternalLinks(True)
        credits_label.setWordWrap(True)
        layout.addWidget(credits_label)
        
        tabs = QTabWidget()
        
        self.app_license_view = QTextEdit()
        self.app_license_view.setReadOnly(True)
        self.app_license_view.setText(
            "MIT License\n\n"
            "Copyright (c) 2026 Ivan Potiienko\n\n"
            "Permission is hereby granted, free of charge, to any person obtaining a copy\n"
            "of this software and associated documentation files (the \"Software\"), to deal\n"
            "in the Software without restriction, including without limitation the rights\n"
            "to use, copy, modify, merge, publish, distribute, sublicense, and/or sell\n"
            "copies of the Software, and to permit persons to whom the Software is\n"
            "furnished to do so, subject to the following conditions:\n\n"
            "The above copyright notice and this permission notice shall be included in all\n"
            "copies or substantial portions of the Software.\n\n"
            "THE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR\n"
            "IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,\n"
            "FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE\n"
            "AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER\n"
            "LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,\n"
            "OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE\n"
            "SOFTWARE."
        )
        tabs.addTab(self.app_license_view, "App License (MIT)")

        self.xdelta_license_view = QTextEdit()
        self.xdelta_license_view.setReadOnly(True)
        
        license_path = get_resource_path(os.path.join("bin", "LICENSE.txt"))
        if os.path.exists(license_path):
            with open(license_path, "r", encoding="utf-8") as f:
                self.xdelta_license_view.setText(f.read())
        else:
            self.xdelta_license_view.setText("License file not found.")
        
        tabs.addTab(self.xdelta_license_view, "xdelta3 License")
        
        layout.addWidget(tabs)
        
        ok_btn = QPushButton("OK")
        ok_btn.clicked.connect(self.accept)
        layout.addWidget(ok_btn)

class ToolsDialog(QDialog):
    def __init__(self, parent):
        super().__init__(parent)
        self.setWindowTitle("Downgrader Tools")
        self.setFixedSize(300, 380)
        self.parent = parent
        
        layout = QVBoxLayout(self)
        
        import platform
        system = platform.system()
        
        path = self.parent.path_edit.text()
        has_path = bool(path and os.path.exists(path))
        has_internet = updater.has_internet()
        has_patches = os.path.exists("Patches/manifest.json")

        self.download_btn = QPushButton("Download/Update Patches")
        self.download_btn.clicked.connect(self.download_patches)
        self.download_btn.setEnabled(has_internet)
        layout.addWidget(self.download_btn)
        
        self.revert_btn = QPushButton("Revert Downgrade (Restore Backups)")
        self.revert_btn.clicked.connect(self.revert_downgrade)
        self.revert_btn.setEnabled(has_path)
        layout.addWidget(self.revert_btn)
        
        self.reg_btn = QPushButton("Fix Registry Path (Windows)")
        self.reg_btn.clicked.connect(self.fix_registry)
        self.reg_btn.setEnabled(has_path)
        layout.addWidget(self.reg_btn)
        
        self.laa_btn = QPushButton("Apply 4GB Patch (LAA)")
        self.laa_btn.clicked.connect(self.apply_laa)
        self.laa_btn.setEnabled(has_path)
        layout.addWidget(self.laa_btn)
        
        self.shortcut_btn = QPushButton("Create Desktop Shortcut")
        self.shortcut_btn.clicked.connect(self.create_shortcut)
        self.shortcut_btn.setEnabled(has_path)
        if system == "Windows":
            layout.addWidget(self.shortcut_btn)

        self.clear_user_btn = QPushButton("Clear User Data (Saves/Settings)")
        self.clear_user_btn.clicked.connect(self.clear_user_data)
        self.clear_user_btn.setEnabled(has_path)
        layout.addWidget(self.clear_user_btn)

        self.cleanup_btn = QPushButton("Cleanup Backups")
        self.cleanup_btn.clicked.connect(self.cleanup_backups)
        self.cleanup_btn.setEnabled(has_path)
        layout.addWidget(self.cleanup_btn)

        if not has_internet and not has_patches:
            for btn in [self.download_btn, self.revert_btn, self.reg_btn, self.laa_btn, self.shortcut_btn, self.clear_user_btn, self.cleanup_btn]:
                if hasattr(self, 'shortcut_btn') or btn != self.shortcut_btn:
                    btn.setEnabled(False)
            layout.addWidget(QLabel("<font color='red'>Internet required for initial setup.</font>"))
        
        layout.addStretch()
        
        row_ext = QHBoxLayout()
        dx_btn = QPushButton("DirectX")
        vc_btn = QPushButton("VC++ Redists")
        
        def open_url(url):
            import webbrowser
            webbrowser.open(url)

        dx_btn.clicked.connect(lambda: open_url("https://www.microsoft.com/en-us/download/details.aspx?id=8109"))
        vc_btn.clicked.connect(lambda: open_url("https://github.com/abbodi1406/vcredist/releases"))
        
        row_ext.addWidget(dx_btn)
        row_ext.addWidget(vc_btn)
        layout.addLayout(row_ext)
        
        close_btn = QPushButton("Close")
        close_btn.clicked.connect(self.accept)
        layout.addWidget(close_btn)

    def fix_registry(self):
        import platform
        if platform.system() != "Windows":
            QMessageBox.information(self, "Info", "Registry fix is only available on Windows.")
            return
        
        path = self.parent.path_edit.text()
        if not path:
            QMessageBox.warning(self, "Warning", "Select game path first.")
            return

        exe_path = os.path.join(path, "gta_sa.exe")
        try:
            import winreg
            key_path = r"SOFTWARE\WOW6432Node\Rockstar Games\Grand Theft Auto San Andreas\Installation"
            try:
                key = winreg.CreateKey(winreg.HKEY_LOCAL_MACHINE, key_path)
            except PermissionError:
                QMessageBox.critical(self, "Permission Error", "Please run the application as Administrator to modify the Registry.")
                return

            winreg.SetValueEx(key, "ExePath", 0, winreg.REG_SZ, exe_path)
            winreg.CloseKey(key)
            QMessageBox.information(self, "Success", "Registry updated successfully.")
        except Exception as e:
            QMessageBox.critical(self, "Error", f"Failed to fix registry: {str(e)}")

    def apply_laa(self):
        path = self.parent.path_edit.text()
        if not path:
            QMessageBox.warning(self, "Warning", "Select game path first.")
            return
            
        exes = ["gta_sa.exe", "gta-sa.exe"]
        applied_to = []
        already_patched = []

        for exe_name in exes:
            exe_path = os.path.join(path, exe_name)
            if not os.path.exists(exe_path):
                continue

            try:
                with open(exe_path, "r+b") as f:
                    f.seek(60)
                    pe_offset = int.from_bytes(f.read(4), "little")
                    f.seek(pe_offset + 4 + 18)
                    characteristics = int.from_bytes(f.read(2), "little")
                    
                    if characteristics & 0x0020:
                        already_patched.append(exe_name)
                        continue
                    
                    characteristics |= 0x0020
                    f.seek(pe_offset + 4 + 18)
                    f.write(characteristics.to_bytes(2, "little"))
                    applied_to.append(exe_name)
            except Exception as e:
                QMessageBox.critical(self, "Error", f"Failed to patch {exe_name}: {str(e)}")

        if not applied_to and not already_patched:
            QMessageBox.critical(self, "Error", "No valid executables (gta_sa.exe or gta-sa.exe) found to patch.")
        elif applied_to:
            msg = f"4GB Patch (LAA) applied successfully to: {', '.join(applied_to)}"
            if already_patched:
                msg += f"\n\nAlready patched: {', '.join(already_patched)}"
            QMessageBox.information(self, "Success", msg)
        else:
            QMessageBox.information(self, "Info", f"4GB Patch (LAA) is already applied to: {', '.join(already_patched)}")

    def create_shortcut(self):
        path = self.parent.path_edit.text()
        if not path:
            QMessageBox.warning(self, "Warning", "Select game path first.")
            return
            
        exe_name = "gta-sa.exe" if os.path.exists(os.path.join(path, "gta-sa.exe")) else "gta_sa.exe"
        exe_path = os.path.abspath(os.path.join(path, exe_name))
        
        try:
            import subprocess
            creationflags = 0
            if platform.system() == "Windows":
                creationflags = subprocess.CREATE_NO_WINDOW
            desktop = os.path.join(os.environ["USERPROFILE"], "Desktop")
            sc_path = os.path.join(desktop, "GTA San Andreas.lnk")
            ps_cmd = f'$s=(New-Object -ComObject WScript.Shell).CreateShortcut("{sc_path}");$s.TargetPath="{exe_path}";$s.WorkingDirectory="{path}";$s.Save()'
            subprocess.run(["powershell", "-Command", ps_cmd], check=True, creationflags=creationflags)
                
            QMessageBox.information(self, "Success", "Desktop shortcut created.")
        except Exception as e:
            QMessageBox.critical(self, "Error", f"Failed to create shortcut: {str(e)}")

    def clear_user_data(self):
        import platform
        system = platform.system()
        game_path = self.parent.path_edit.text()
        
        if system == "Windows":
            user_path = os.path.join(os.environ["USERPROFILE"], "Documents", "GTA San Andreas User Files")
        elif system == "Linux":
            if not game_path:
                QMessageBox.warning(self, "Warning", "Select game path first to locate Linux user data.")
                return
            
            steamapps_dir = os.path.dirname(os.path.dirname(game_path))
            user_path = os.path.join(steamapps_dir, "compatdata", "12120", "pfx", "drive_c", "users", "steamuser", "Documents", "GTA San Andreas User Files")
            
            if not os.path.exists(user_path):
                user_path = os.path.expanduser("~/Documents/GTA San Andreas User Files")
        else:
            return

        if os.path.exists(user_path):
            reply = QMessageBox.question(self, "Confirm Delete", 
                "Are you sure you want to delete all user data (saves, settings, gallery)?",
                QMessageBox.Yes | QMessageBox.No)
            
            if reply == QMessageBox.Yes:
                import shutil
                try:
                    shutil.rmtree(user_path)
                    QMessageBox.information(self, "Success", "User data cleared.")
                except Exception as e:
                    QMessageBox.critical(self, "Error", f"Failed to clear user data: {str(e)}")
        else:
            QMessageBox.information(self, "Info", "No user data found.")

    def download_patches(self):
        if not updater.has_internet():
            QMessageBox.critical(self, "Error", "Internet connection required to download patches.")
            return

        self.accept()
        icloud_url = "https://www.icloud.com/iclouddrive/0afGK6zDBog_0drwp6YZoDLIg#Patches"
        target_dir = os.path.abspath("Patches")
        dlg = DownloadDialog(icloud_url, target_dir)
        dlg.exec()
        if dlg.success:
            QMessageBox.information(self, "Success", "Patches downloaded successfully.")
            if self.parent.path_edit.text():
                self.parent.scan_directory(self.parent.path_edit.text())

    def revert_downgrade(self):
        self.accept()
        self.parent.revert_downgrade()

    def cleanup_backups(self):
        path = self.parent.path_edit.text()
        if not path:
            QMessageBox.warning(self, "Warning", "Select game path first.")
            return
            
        backup_dir = os.path.join(path, "backups")
        if os.path.exists(backup_dir):
            reply = QMessageBox.question(self, "Confirm Cleanup", 
                "Are you sure you want to delete all backups? This cannot be undone.",
                QMessageBox.Yes | QMessageBox.No)
            
            if reply == QMessageBox.Yes:
                import shutil
                try:
                    shutil.rmtree(backup_dir)
                    QMessageBox.information(self, "Success", "Backups cleaned up.")
                except Exception as e:
                    QMessageBox.critical(self, "Error", f"Failed to cleanup backups: {str(e)}")
        else:
            QMessageBox.information(self, "Info", "No backups found.")

def calculate_md5(file_path):
    hash_md5 = hashlib.md5()
    try:
        with open(file_path, "rb") as f:
            for chunk in iter(lambda: f.read(4096), b""):
                hash_md5.update(chunk)
        return hash_md5.hexdigest()
    except Exception:
        return None

def get_resource_path(relative_path):
    try:
        base_path = sys._MEIPASS
    except Exception:
        base_path = os.path.dirname(os.path.abspath(__file__))
    return os.path.join(base_path, relative_path)

def find_game_path():
    import platform
    system = platform.system()
    
    if system == "Windows":
        try:
            import winreg
            registry_paths = [
                (winreg.HKEY_LOCAL_MACHINE, r"SOFTWARE\WOW6432Node\Rockstar Games\Grand Theft Auto San Andreas\Installation", "ExePath"),
                (winreg.HKEY_LOCAL_MACHINE, r"SOFTWARE\Rockstar Games\Grand Theft Auto San Andreas\Installation", "ExePath"),
                (winreg.HKEY_LOCAL_MACHINE, r"SOFTWARE\WOW6432Node\Valve\Steam", "InstallPath"),
                (winreg.HKEY_CURRENT_USER, r"Software\Valve\Steam", "SteamPath"),
            ]
            
            for root, key_path, value_name in registry_paths:
                try:
                    key = winreg.OpenKey(root, key_path)
                    val, _ = winreg.QueryValueEx(key, value_name)
                    winreg.CloseKey(key)
                    
                    if r"Valve\Steam" in key_path:
                        val = os.path.join(val, "steamapps", "common", "Grand Theft Auto San Andreas")
                    
                    if os.path.exists(val):
                        if os.path.isfile(val):
                            val = os.path.dirname(val)
                        if any(os.path.exists(os.path.join(val, e)) for e in ["gta_sa.exe", "gta-sa.exe"]):
                            return val
                except Exception:
                    continue
        except ImportError:
            pass
                
    elif system == "Linux":
        home = os.path.expanduser("~")
        common_paths = [
            os.path.join(home, ".local/share/Steam/steamapps/common/Grand Theft Auto San Andreas"),
            os.path.join(home, ".steam/steam/steamapps/common/Grand Theft Auto San Andreas"),
            os.path.join(home, ".steam/root/steamapps/common/Grand Theft Auto San Andreas"),
            os.path.join(home, ".var/app/com.valvesoftware.Steam/data/Steam/steamapps/common/Grand Theft Auto San Andreas"),
            "/run/media/mmcblk0p1/steamapps/common/Grand Theft Auto San Andreas",
        ]
        for path in common_paths:
            if os.path.exists(path) and any(os.path.exists(os.path.join(path, e)) for e in ["gta_sa.exe", "gta-sa.exe"]):
                return path
                
    return ""

VERSION_HASHES = {
    "170b3a9108687b26da2d8901c6948a18": "v1.0 US (Hoodlum)",
    "2b5066bd4097ac2944ce6a9cf8fe5677": "v1.0 US (Hoodlum + LAA Patch)",
    "667f799c4ba8c9e1054fccaea6d4259b": "v1.0 US (Compact)",
    "6c6160da9b175b66cf9127c86be57bf7": "v1.0 EU",
    "49dd417760484a18017805df46b308b8": "v1.0 EU (Alt)",
    "9f2d711dbf1fbbcda5ff9418a2cc1ef5": "v1.01 US",
    "25405921d1c47747fd01fd0bfe0a05ae": "v1.01 EU",
    "d9cb35c898d3298ca904a63e10ee18d9": "NewSteam R2 (German)",
    "5bfd4dd83989a8264de4b8e771f237fd": "NewSteam R2",
}

class ScannerThread(QThread):
    progress = Signal(int, int)
    finished = Signal(list, str, bool)

    def __init__(self, path, manifest_path):
        super().__init__()
        self.path = path
        self.manifest_path = manifest_path

    def run(self):
        detected_version = "Unknown"
        is_readonly = False
        
        try:
            test_file = os.path.join(self.path, ".downgrader_test")
            with open(test_file, 'w') as f:
                f.write("test")
            os.remove(test_file)
        except Exception:
            is_readonly = True

        for exe_name in ["gta_sa.exe", "gta-sa.exe"]:
            exe_path = os.path.join(self.path, exe_name)
            if os.path.exists(exe_path):
                h = calculate_md5(exe_path)
                if h in VERSION_HASHES:
                    detected_version = VERSION_HASHES[h]
                    break
                else:
                    detected_version = f"Custom/Unknown ({h[:8]})"

        try:
            with open(self.manifest_path, 'r') as f:
                manifest = json.load(f)
        except Exception:
            self.finished.emit([], detected_version, is_readonly)
            return

        files_to_check = manifest.get("files", [])
        total = len(files_to_check)
        results = []

        for i, file_info in enumerate(files_to_check):
            rel_path = file_info["path"]
            full_path = os.path.join(self.path, rel_path)
            
            if rel_path in ["gta_sa.exe", "gta-sa.exe"] and not os.path.exists(full_path):
                alt_name = "gta-sa.exe" if rel_path == "gta_sa.exe" else "gta_sa.exe"
                alt_path = os.path.join(self.path, alt_name)
                if os.path.exists(alt_path):
                    full_path = alt_path

            current_hash = calculate_md5(full_path)
            target_hash = file_info["target_hash"]
            source_hash = file_info["source_hash"]

            status = "Ready"
            needs_patch = "No"

            is_already_patched = (current_hash == target_hash)
            is_laa = (rel_path in ["gta_sa.exe", "gta-sa.exe"] and current_hash == "2b5066bd4097ac2944ce6a9cf8fe5677")

            if is_already_patched or is_laa:
                status = "Already Downgraded"
                if is_laa:
                    status += " (LAA)"
                needs_patch = "No"
            elif current_hash == source_hash:
                status = "Original (Needs Patch)"
                needs_patch = "Yes"
            elif current_hash is None:
                status = "Missing"
                needs_patch = "N/A"
            else:
                status = "Modified"
                needs_patch = "Yes (Force)"

            results.append({
                "path": rel_path,
                "needs_patch": needs_patch,
                "status": status,
                "current_hash": current_hash or "N/A",
                "target_hash": target_hash
            })
            self.progress.emit(i + 1, total)

        self.finished.emit(results, detected_version, is_readonly)

class PatchThread(QThread):
    file_progress = Signal(int, str, str)
    finished = Signal(int, int)

    def __init__(self, game_path, manifest, xdelta_bin, patches_dir):
        super().__init__()
        self.game_path = game_path
        self.manifest = manifest
        self.xdelta_bin = xdelta_bin
        self.patches_dir = patches_dir

    def run(self):
        import subprocess
        import shutil
        import platform

        creationflags = 0
        if platform.system() == "Windows":
            creationflags = subprocess.CREATE_NO_WINDOW

        success_count = 0
        fail_count = 0
        
        backup_dir = os.path.join(self.game_path, "backups")
        os.makedirs(backup_dir, exist_ok=True)

        files = self.manifest.get("files", [])
        for i, file_info in enumerate(files):
            rel_path = file_info["path"]
            action = file_info.get("action", "patch")
            
            target_file = os.path.join(self.game_path, rel_path)
            
            if rel_path in ["gta_sa.exe", "gta-sa.exe"] and not os.path.exists(target_file):
                alt_name = "gta-sa.exe" if rel_path == "gta_sa.exe" else "gta_sa.exe"
                alt_path = os.path.join(self.game_path, alt_name)
                if os.path.exists(alt_path):
                    target_file = alt_path

            target_hash = file_info.get("target_hash")
            current_hash = calculate_md5(target_file)
            is_laa = (rel_path in ["gta_sa.exe", "gta-sa.exe"] and current_hash == "2b5066bd4097ac2944ce6a9cf8fe5677")
            
            if current_hash == target_hash or is_laa:
                self.file_progress.emit(i, "Already Patched", "")
                
                if rel_path in ["gta_sa.exe", "gta-sa.exe"]:
                    alt_name = "gta-sa.exe" if target_file.endswith("gta_sa.exe") else "gta_sa.exe"
                    alt_path = os.path.join(self.game_path, alt_name)
                    if not os.path.exists(alt_path):
                        shutil.copy2(target_file, alt_path)

                success_count += 1
                continue

            self.file_progress.emit(i, "Backup & Patching...", "")
            
            try:
                if os.path.exists(target_file):
                    rel_dir = os.path.dirname(rel_path)
                    dest_backup_dir = os.path.join(backup_dir, rel_dir)
                    os.makedirs(dest_backup_dir, exist_ok=True)
                    shutil.copy2(target_file, os.path.join(backup_dir, rel_path))

                if action == "copy":
                    source_patch = os.path.join(self.patches_dir, "gta_sa.exe")
                    shutil.copy2(source_patch, target_file)
                    
                    if rel_path in ["gta_sa.exe", "gta-sa.exe"]:
                        alt_name = "gta-sa.exe" if target_file.endswith("gta_sa.exe") else "gta_sa.exe"
                        alt_path = os.path.join(self.game_path, alt_name)
                        shutil.copy2(source_patch, alt_path)
                        
                    success_count += 1
                    self.file_progress.emit(i, "Success (Copy)", "")
                else:
                    patch_file = os.path.join(self.patches_dir, f"{rel_path}.xdelta")
                    if not os.path.exists(patch_file):
                        self.file_progress.emit(i, "Failed", "Patch file missing")
                        fail_count += 1
                        continue
                    
                    temp_output = target_file + ".tmp"
                    cmd = [self.xdelta_bin, "-d", "-s", target_file, patch_file, temp_output]
                    result = subprocess.run(cmd, capture_output=True, text=True, creationflags=creationflags)
                    
                    if result.returncode == 0:
                        os.replace(temp_output, target_file)
                        
                        if rel_path in ["gta_sa.exe", "gta-sa.exe"]:
                            alt_name = "gta-sa.exe" if target_file.endswith("gta_sa.exe") else "gta_sa.exe"
                            alt_path = os.path.join(self.game_path, alt_name)
                            shutil.copy2(target_file, alt_path)

                        success_count += 1
                        self.file_progress.emit(i, "Success", "")
                    else:
                        fail_count += 1
                        self.file_progress.emit(i, "Failed", "xdelta error")
                        if os.path.exists(temp_output): os.remove(temp_output)
            except Exception as e:
                self.file_progress.emit(i, "Error", str(e))
                fail_count += 1

        self.finished.emit(success_count, fail_count)

    def status_bar_msg(self, msg):
        pass

class DowngraderApp(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("GTA SA Open Downgrader")
        self.resize(850, 600)
        
        self.manifest_data = None
        self.detected_appid = None
        self.is_v10_us = False

        self.init_ui()
        
        auto_path = find_game_path()
        if auto_path:
            self.path_edit.setText(auto_path)

    def check_patches_and_start(self):
        if updater.has_internet():
            latest, assets = updater.check_for_updates()
            if latest:
                system = platform.system()
                download_url = None
                for asset in assets:
                    if system == "Windows" and asset["name"].endswith(".exe"):
                        if updater.is_offline():
                            if "installer" in asset["name"].lower():
                                download_url = asset["browser_download_url"]
                                break
                        else:
                            if "installer" not in asset["name"].lower():
                                download_url = asset["browser_download_url"]
                                break
                    elif system == "Linux" and asset["name"].lower().endswith(".appimage"):
                        if updater.is_offline():
                            if "offline" in asset["name"].lower():
                                download_url = asset["browser_download_url"]
                                break
                        else:
                            if "offline" not in asset["name"].lower():
                                download_url = asset["browser_download_url"]
                                break
                
                if not download_url and assets:
                    for asset in assets:
                        if system == "Windows" and asset["name"].endswith(".exe"):
                            download_url = asset["browser_download_url"]
                            break
                        elif system == "Linux" and asset["name"].lower().endswith(".appimage"):
                            download_url = asset["browser_download_url"]
                            break

                if download_url:
                    if updater.is_offline():
                        self.update_btn.setVisible(True)
                        self.update_btn.clicked.connect(lambda: self.trigger_update(latest, download_url))
                        self.status_bar.showMessage(f"Notification: New version {latest} available!")
                    else:
                        reply = QMessageBox.question(self, "Update Available", 
                            f"A new version ({latest}) is available. Would you like to update now?",
                            QMessageBox.Yes | QMessageBox.No)
                        if reply == QMessageBox.Yes:
                            self.status_bar.showMessage("Downloading update...")
                            updater.run_update_script(download_url)
                            return
                else:
                    QMessageBox.warning(self, "Update Error", "Could not find a matching download for your platform.")

        self.resolved_manifest_path = "Patches/manifest.json"
        if not os.path.exists(self.resolved_manifest_path):
            self.resolved_manifest_path = get_resource_path("Patches/manifest.json")
            
        has_patches = os.path.exists(self.resolved_manifest_path)
        has_internet = updater.has_internet()

        if not has_patches:
            if not has_internet:
                QMessageBox.critical(self, "Fatal Error", 
                    "Patches are missing and no internet connection was detected.\n\n"
                    "The application cannot function without patch assets or internet to download them.")
                sys.exit(1)

            reply = QMessageBox.question(self, "Patches Missing", 
                "Patches folder not found. Would you like to download them from iCloud?",
                QMessageBox.Yes | QMessageBox.No)
            
            if reply == QMessageBox.Yes:
                icloud_url = "https://www.icloud.com/iclouddrive/0afGK6zDBog_0drwp6YZoDLIg#Patches"
                target_dir = os.path.abspath("Patches")
                
                dlg = DownloadDialog(icloud_url, target_dir)
                dlg.exec()
                
                if dlg.success:
                    if os.path.exists("Patches/manifest.json"):
                        self.resolved_manifest_path = "Patches/manifest.json"
                        self.show()
                        if self.path_edit.text():
                            self.scan_directory(self.path_edit.text())
                    else:
                        QMessageBox.critical(self, "Error", "Download finished but manifest.json is still missing.")
                        sys.exit(1)
                else:
                    QMessageBox.critical(self, "Error", "Failed to download patches.")
                    sys.exit(1)
            else:
                self.status_bar.showMessage("Warning: Patches missing. Downgrade will be disabled.")
                self.show()
                if self.path_edit.text():
                    self.scan_directory(self.path_edit.text())
        else:
            self.show()
            if self.path_edit.text():
                self.scan_directory(self.path_edit.text())

        if not has_internet:
            self.status_bar.showMessage("Offline Mode: Mod installation and updates disabled.")
            for cb in self.mods.values():
                cb.setEnabled(False)
                cb.setToolTip("Requires internet connection.")
            self.install_mods_only_btn.setEnabled(False)

    def init_ui(self):
        central_widget = QWidget()
        self.setCentralWidget(central_widget)
        main_layout = QVBoxLayout(central_widget)
        main_layout.setContentsMargins(10, 10, 10, 10)
        main_layout.setSpacing(8)

        header_layout = QHBoxLayout()
        self.update_btn = QPushButton("UPDATE AVAILABLE")
        self.update_btn.setVisible(False)
        self.update_btn.setStyleSheet("""
            QPushButton { 
                background-color: #f44336; color: white; font-weight: bold; 
                font-size: 10px; border-radius: 4px; padding: 2px 6px;
            }
        """)
        header_layout.addWidget(self.update_btn)
        
        version_label = QLabel(updater.CURRENT_VERSION)
        header_layout.addStretch()
        header_layout.addWidget(version_label, 0, Qt.AlignRight)
        main_layout.addLayout(header_layout)

        path_layout = QHBoxLayout()
        path_layout.addWidget(QLabel("Path:"))
        self.path_edit = QLineEdit()
        self.path_edit.setPlaceholderText("Select GTA San Andreas installation directory...")
        self.browse_btn = QPushButton("Browse")
        self.browse_btn.setFixedWidth(80)
        self.browse_btn.clicked.connect(self.browse_path)
        path_layout.addWidget(self.path_edit)
        path_layout.addWidget(self.browse_btn)
        main_layout.addLayout(path_layout)

        info_layout = QHBoxLayout()
        self.version_info = QLabel("Version: Unknown")
        self.readonly_info = QLabel("Read-Only: Unknown")
        info_layout.addWidget(self.version_info)
        info_layout.addStretch()
        info_layout.addWidget(self.readonly_info)
        main_layout.addLayout(info_layout)

        mod_group = QGroupBox("Essential Modifications")
        mod_grid = QGridLayout()
        mod_grid.setSpacing(4)
        self.mods = {
            "ASI Loader": QCheckBox("ASI Loader"),
            "ModLoader": QCheckBox("ModLoader"),
            "SilentPatch": QCheckBox("SilentPatch"),
            "Widescreen Fixes": QCheckBox("Widescreen Fixes"),
            "SkyGFX": QCheckBox("SkyGFX"),
            "Frontend Mods": QCheckBox("Frontend Mods"),
            "Framerate Vigilante (60fps fix)": QCheckBox("Framerate Vigilante (60fps fix)"),
            "GInput": QCheckBox("GInput"),
            "Project 2DFX": QCheckBox("Project 2DFX"),
        }
        
        row, col = 0, 0
        for name, cb in self.mods.items():
            cb.setStyleSheet("font-size: 11px;")
            cb.setEnabled(False)
            cb.stateChanged.connect(self.handle_mod_dependencies)
            mod_grid.addWidget(cb, row, col)
            col += 1
            if col > 4:
                col = 0
                row += 1
        
        self.install_mods_only_btn = QPushButton("Install Selected Mods Only")
        self.install_mods_only_btn.setFixedHeight(24)
        self.install_mods_only_btn.setStyleSheet("font-size: 10px;")
        self.install_mods_only_btn.clicked.connect(self.install_only_mods_clicked)
        mod_grid.addWidget(self.install_mods_only_btn, row, col, 1, 5 - col)
        
        mod_group.setLayout(mod_grid)
        main_layout.addWidget(mod_group)

        self.table = QTableWidget()
        self.table.setColumnCount(5)
        self.table.setHorizontalHeaderLabels(["File", "Patch", "Status", "Current MD5", "Target MD5"])
        self.table.horizontalHeader().setSectionResizeMode(QHeaderView.Stretch)
        self.table.setStyleSheet("font-size: 10px;")
        self.table.verticalHeader().setDefaultSectionSize(20)
        main_layout.addWidget(self.table)

        footer_layout = QHBoxLayout()
        self.exit_btn = QPushButton("Exit")
        self.exit_btn.clicked.connect(self.close)
        
        self.info_btn = QPushButton("Info")
        self.info_btn.clicked.connect(self.show_about)
        self.tools_btn = QPushButton("Tools")
        self.tools_btn.clicked.connect(self.show_tools)
        self.revert_btn = QPushButton("Revert")
        self.revert_btn.clicked.connect(self.revert_downgrade)
        self.downgrade_btn = QPushButton("START DOWNGRADE")
        self.downgrade_btn.setStyleSheet("""
            QPushButton { font-weight: bold; background-color: #2e7d32; color: white; height: 28px; }
            QPushButton:disabled { background-color: #555555; color: #aaaaaa; }
        """)
        self.downgrade_btn.clicked.connect(self.start_downgrade)

        footer_layout.addWidget(self.exit_btn)
        footer_layout.addStretch()
        footer_layout.addWidget(self.info_btn)
        footer_layout.addWidget(self.tools_btn)
        footer_layout.addWidget(self.revert_btn)
        footer_layout.addWidget(self.downgrade_btn)
        main_layout.addLayout(footer_layout)

        self.revert_btn.setEnabled(False)
        self.downgrade_btn.setEnabled(False)
        self.install_mods_only_btn.setEnabled(False)

        self.status_bar = QStatusBar()
        self.status_bar.setStyleSheet("font-size: 10px;")
        self.setStatusBar(self.status_bar)
        self.status_bar.showMessage("Ready")

    def handle_mod_dependencies(self, _):
        self.mods["ASI Loader"].blockSignals(True)
        self.mods["ModLoader"].blockSignals(True)
        
        dependent_mods = [k for k in self.mods.keys() if k not in ["ASI Loader", "ModLoader"]]
        any_dependent_checked = any(self.mods[k].isChecked() for k in dependent_mods)
        
        if any_dependent_checked:
            self.mods["ASI Loader"].setChecked(True)
            self.mods["ASI Loader"].setEnabled(False)
            self.mods["ModLoader"].setChecked(True)
            self.mods["ModLoader"].setEnabled(False)
        else:
            self.mods["ModLoader"].setEnabled(True)
            
            if self.mods["ModLoader"].isChecked():
                self.mods["ASI Loader"].setChecked(True)
                self.mods["ASI Loader"].setEnabled(False)
            else:
                self.mods["ASI Loader"].setEnabled(True)
                self.mods["ASI Loader"].setChecked(False)
                self.mods["ModLoader"].setChecked(False)

        self.mods["ASI Loader"].blockSignals(False)
        self.mods["ModLoader"].blockSignals(False)

    def trigger_update(self, version, url):
        reply = QMessageBox.question(self, "Confirm Update", 
            f"Would you like to update to {version} now? The application will restart.",
            QMessageBox.Yes | QMessageBox.No)
        if reply == QMessageBox.Yes:
            self.status_bar.showMessage("Downloading update...")
            updater.run_update_script(url)

    def show_about(self):
        dlg = AboutDialog()
        dlg.exec()

    def show_tools(self):
        dlg = ToolsDialog(self)
        dlg.exec()

    def revert_downgrade(self):
        path = self.path_edit.text()
        if not path:
            QMessageBox.warning(self, "Warning", "Please select game path first.")
            return

        backup_dir = os.path.join(path, "backups")
        if not os.path.exists(backup_dir):
            QMessageBox.warning(self, "No Backups", "No backups found to revert.")
            return

        reply = QMessageBox.question(self, "Confirm Revert", 
            "Are you sure you want to revert to the original files? This will overwrite current game files.",
            QMessageBox.Yes | QMessageBox.No)
        
        if reply == QMessageBox.Yes:
            import shutil
            self.status_bar.showMessage("Reverting files...")
            try:
                for root, dirs, files in os.walk(backup_dir):
                    for file in files:
                        backup_file = os.path.join(root, file)
                        rel_path = os.path.relpath(backup_file, backup_dir)
                        target_file = os.path.join(path, rel_path)
                        
                        os.makedirs(os.path.dirname(target_file), exist_ok=True)
                        shutil.copy2(backup_file, target_file)
                
                QMessageBox.information(self, "Success", "Revert complete!")
                self.scan_directory(path)
            except Exception as e:
                QMessageBox.critical(self, "Error", f"Failed to revert: {str(e)}")

    def browse_path(self):
        dir_path = QFileDialog.getExistingDirectory(self, "Select GTA SA Directory")
        if dir_path:
            self.path_edit.setText(dir_path)
            self.scan_directory(dir_path)

    def scan_directory(self, path):
        if not hasattr(self, 'resolved_manifest_path'):
            self.resolved_manifest_path = "Patches/manifest.json"
            if not os.path.exists(self.resolved_manifest_path):
                self.resolved_manifest_path = get_resource_path("Patches/manifest.json")

        if not os.path.exists(self.resolved_manifest_path):
            QMessageBox.warning(self, "Warning", "Patches missing. Please run Tools -> Download Patches or restart the app.")
            return

        self.status_bar.showMessage("Scanning files...")
        self.scanner = ScannerThread(path, self.resolved_manifest_path)
        self.scanner.progress.connect(lambda cur, tot: self.status_bar.showMessage(f"Scanning: {cur}/{tot}"))
        self.scanner.finished.connect(self.update_table)
        self.scanner.start()

        if linux_tools.is_linux():
            apps = linux_tools.get_steam_apps()
            if apps:
                self.detected_appid = apps[0]["appid"]
                self.status_bar.showMessage(f"Linux detected. Steam AppID: {self.detected_appid}")

    def update_table(self, results, detected_version, is_readonly):
        self.table.setRowCount(0)
        different_count = 0
        self.is_v10_us = "v1.0 US" in detected_version
        
        if is_readonly:
            self.readonly_info.setText("Read-Only: Yes")
            self.readonly_info.setStyleSheet("color: red; font-weight: bold;")
            self.downgrade_btn.setEnabled(False)
            self.downgrade_btn.setToolTip("Cannot downgrade: Game directory is read-only.")
        else:
            self.readonly_info.setText("Read-Only: No")
            self.readonly_info.setStyleSheet("color: green;")
            self.downgrade_btn.setEnabled(True)
            self.downgrade_btn.setToolTip("")

        installed_mods = []
        game_path = self.path_edit.text()
        config_path = os.path.join(game_path, "modloader", ".downgrader")
        if os.path.exists(config_path):
            try:
                with open(config_path, 'r') as f:
                    data = json.load(f)
                    installed_mods = data.get("installed_mods", [])
            except Exception:
                pass

        for row, data in enumerate(results):
            self.table.insertRow(row)
            self.table.setItem(row, 0, QTableWidgetItem(data["path"]))
            self.table.setItem(row, 1, QTableWidgetItem(data["needs_patch"]))
            self.table.setItem(row, 2, QTableWidgetItem(data["status"]))
            self.table.setItem(row, 3, QTableWidgetItem(data["current_hash"]))
            self.table.setItem(row, 4, QTableWidgetItem(data["target_hash"]))
            
            if data["needs_patch"] == "Yes":
                different_count += 1
        
        self.version_info.setText(f"Version: {detected_version}")
        if different_count > 0:
            self.status_bar.showMessage(f"Scan complete. {different_count} files need patching.")
        else:
            self.status_bar.showMessage("Scan complete. Game is already v1.0 US.")

        self.revert_btn.setEnabled(True)
        self.install_mods_only_btn.setEnabled(updater.has_internet())

        for name, cb in self.mods.items():
            cb.setEnabled(updater.has_internet())
            cb.setToolTip("" if updater.has_internet() else "Requires internet connection.")
            if name in installed_mods:
                cb.setChecked(True)

    def start_downgrade(self):
        path = self.path_edit.text()
        if not path:
            QMessageBox.warning(self, "Warning", "Please select game path first.")
            return

        if not os.path.exists(self.resolved_manifest_path):
            QMessageBox.critical(self, "Error", "Manifest not found.")
            return

        try:
            with open(self.resolved_manifest_path, 'r') as f:
                manifest = json.load(f)
        except Exception as e:
            QMessageBox.critical(self, "Error", f"Failed to load manifest: {str(e)}")
            return

        patches_dir = os.path.dirname(self.resolved_manifest_path)
        self.status_bar.showMessage("Starting downgrade...")
        
        import platform
        if platform.system() == "Windows":
            xdelta_bin = get_resource_path(os.path.join("bin", "xdelta3.exe"))
        else:
            xdelta_bin = get_resource_path(os.path.join("bin", "xdelta3_linux"))

        if not os.path.exists(xdelta_bin):
            xdelta_bin = "xdelta3"
            
        try:
            import subprocess
            creationflags = 0
            if platform.system() == "Windows":
                creationflags = subprocess.CREATE_NO_WINDOW
            subprocess.run([xdelta_bin, "-V"], capture_output=True, creationflags=creationflags)
        except FileNotFoundError:
            QMessageBox.critical(self, "Error", f"xdelta3 binary not found at {xdelta_bin} or in PATH.")
            return

        self.downgrade_btn.setEnabled(False)
        self.browse_btn.setEnabled(False)
        
        self.patch_thread = PatchThread(path, manifest, xdelta_bin, patches_dir)
        self.patch_thread.file_progress.connect(self.update_file_status)
        self.patch_thread.finished.connect(self.handle_patch_finished)
        self.patch_thread.start()

    def update_file_status(self, row, status, message):
        self.table.setItem(row, 2, QTableWidgetItem(status))
        if message:
            self.table.setItem(row, 2, QTableWidgetItem(f"{status} ({message})"))
        self.table.scrollToItem(self.table.item(row, 0))

    def handle_patch_finished(self, success_count, fail_count):
        self.downgrade_btn.setEnabled(True)
        self.browse_btn.setEnabled(True)
        path = self.path_edit.text()

        if fail_count == 0:
            selected = [name for name, cb in self.mods.items() if cb.isChecked()]
            if selected:
                self.install_selected_mods(path, selected)
            else:
                QMessageBox.information(self, "Success", f"Downgrade complete! {success_count} files processed successfully.")
        else:
            QMessageBox.warning(self, "Finished with Errors", f"Downgrade finished. Success: {success_count}, Failed: {fail_count}")

        self.scan_directory(path)

    def install_only_mods_clicked(self):
        path = self.path_edit.text()
        if not path:
            QMessageBox.warning(self, "Warning", "Please select game path first.")
            return

        selected = [name for name, cb in self.mods.items() if cb.isChecked()]
        if not selected:
            QMessageBox.warning(self, "Warning", "No mods selected.")
            return

        self.install_selected_mods(path, selected)

    def install_selected_mods(self, game_path, selected_mods):
        if not updater.has_internet():
            QMessageBox.critical(self, "Error", "Internet connection required to install mods.")
            return

        dlg = ModInstallDialog(game_path, selected_mods)
        dlg.exec()
        
        if dlg.success:
            if linux_tools.is_linux() and "ASI Loader" in selected_mods:
                linux_dlg = LinuxLaunchOptionsDialog()
                linux_dlg.exec()
            
            QMessageBox.information(self, "Complete", "Mod installation complete.")
        else:
            QMessageBox.critical(self, "Error", f"Failed to install mods:\n{dlg.error_message}")

if __name__ == "__main__":

    app = QApplication(sys.argv)
    app.setStyle("Fusion")
    
    icon_path = get_resource_path(os.path.join("assets", "icon.ico"))
    if os.path.exists(icon_path):
        app.setWindowIcon(QIcon(icon_path))

    window = DowngraderApp()

    window.check_patches_and_start()

    sys.exit(app.exec())