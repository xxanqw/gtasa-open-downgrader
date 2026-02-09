import subprocess
import os

def get_steam_apps():
    try:
        result = subprocess.run(['protontricks', '-l'], capture_output=True, text=True)
        apps = []
        for line in result.stdout.splitlines():
            if "Grand Theft Auto: San Andreas" in line:
                parts = line.split('(')
                if len(parts) > 1:
                    appid = parts[-1].strip(')')
                    apps.append({"name": line.strip(), "appid": appid})
        return apps
    except FileNotFoundError:
        return []

def install_exe_via_protontricks(appid, exe_path):
    if not os.path.exists(exe_path):
        return False, "EXE file not found."
    
    try:
        subprocess.Popen(['protontricks', appid, exe_path])
        return True, "Installation started via Protontricks."
    except Exception as e:
        return False, str(e)

def is_linux():
    import platform
    return platform.system() == "Linux"
