import os
import sys
import platform
import requests
import subprocess
import time

CURRENT_VERSION = "v0.1.1"
REPO_URL = "https://api.github.com/repos/xxanqw/gtasa-open-downgrader/releases/latest"

def has_internet():
    try:
        requests.get("https://1.1.1.1", timeout=3)
        return True
    except (requests.ConnectionError, requests.Timeout):
        return False

def check_for_updates():
    if not has_internet():
        return None, None

    try:
        response = requests.get(REPO_URL, timeout=5)
        response.raise_for_status()
        data = response.json()
        latest_version = data.get("tag_name")
        
        if latest_version and latest_version != CURRENT_VERSION:
            return latest_version, data.get("assets", [])
    except Exception:
        pass
    
    return None, None

def run_update_script(download_url):
    system = platform.system()
    temp_dir = os.path.dirname(os.path.abspath(sys.executable if getattr(sys, 'frozen', False) else __file__))
    
    appimage_path = os.environ.get("APPIMAGE")
    
    if system == "Windows":
        exe_path = sys.executable
        script_path = os.path.join(temp_dir, "update.ps1")
        script_content = f"""
Start-Sleep -Seconds 2
Invoke-WebRequest -Uri "{download_url}" -OutFile "{exe_path}.new"
Move-Item -Path "{exe_path}.new" -Destination "{exe_path}" -Force
Start-Process "{exe_path}"
Remove-Item $MyInvocation.MyCommand.Path
"""
        with open(script_path, "w") as f:
            f.write(script_content)
        
        subprocess.Popen(["powershell", "-ExecutionPolicy", "Bypass", "-File", script_path], 
                         creationflags=subprocess.CREATE_NO_WINDOW if hasattr(subprocess, 'CREATE_NO_WINDOW') else 0)
        sys.exit(0)
    
    else:
        exe_path = appimage_path if appimage_path else sys.executable
        script_path = os.path.join(temp_dir, "update.sh")
        script_content = f"""#!/bin/bash
sleep 2
curl -L "{download_url}" -o "{exe_path}.new"
chmod +x "{exe_path}.new"
mv "{exe_path}.new" "{exe_path}"
"{exe_path}" &
rm "$0"
"""
        with open(script_path, "w") as f:
            f.write(script_content)
        
        os.chmod(script_path, 0o755)
        subprocess.Popen(["/bin/bash", script_path])
        sys.exit(0)

def get_bundle_path(relative_path):
    try:
        base_path = sys._MEIPASS
    except Exception:
        base_path = os.path.dirname(os.path.abspath(__file__))
    return os.path.join(base_path, relative_path)

def is_offline():
    return os.path.exists(get_bundle_path("Patches/manifest.json"))
