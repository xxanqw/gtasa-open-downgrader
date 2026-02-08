# GTA SA Open Downgrader

This project was initiated because the original RockstarNexus Downgrader was easy and intuitive to use, and we wanted to replicate that experience in an open-source format.

The original Hoodlum release was used to create the patches. Currently, it supports the NewSteam R2 global version only.

If someone wants to help with creating patches for other versions of the game, I am open to this! You can contact me via the email in my profile or via GitHub issues (please provide a URL to your patches for the other game versions).

## Patches
- **Source Version:** NewSteam R2 Global
- **Target Version:** v1.0 US (Hoodlum)
- **Download:** [Patches.zip (iCloud)](https://www.icloud.com/iclouddrive/0afGK6zDBog_0drwp6YZoDLIg#Patches)

## Features
- **Automated Game Detection:** Automatically locates your GTA San Andreas installation on Windows (Registry) and Linux (common Steam library paths).
- **One-Click Downgrade:** Automated downgrading to v1.0 US (Hoodlum) with `xdelta3` technology.
- **Essential Modifications:** Integrated downloader and installer for several mods. Note: Some mods are hosted on my own fileserver (`fs.xserv.pp.ua`) because they required specific structural changes or presets to be properly installed by the downgrader.
    - ASI Loader & ModLoader
    - SilentPatch & GInput
    - Widescreen Fixes & Project 2DFX
    - SkyGFX (with multiple presets)
    - Framerate Vigilante (60fps fix)
    - Frontend Mods
- **Mod Tracking:** Automatically detects already installed mods using a persistent configuration file.
- **Downgrader Tools:**
    - **4GB Patch (LAA):** Optimize your executable to use more memory and prevent crashes.
    - **Registry Fix:** Repair game installation paths in the Windows Registry.
    - **Revert System:** Automatic backups before patching allow for a full restoration of original files.
    - **User Data Management:** Quickly clear saves and settings to troubleshoot game issues.
- **Cross-Platform:** Full support for Windows and Linux (including Proton/Steam Deck specific optimizations).
- **Safety First:** Detects read-only directories and provides necessary Steam launch options for Linux users.

### Linux Execution
On Linux, the application is distributed as a single executable. Simply grant it execution permissions and launch it:
```bash
chmod +x gta-sa-downgrader-linux
./gta-sa-downgrader-linux
```

### Steam Deck Support
The application is fully compatible with the Steam Deck. It automatically detects game installations on both internal storage and SD cards. Since Steam games are stored in the writable `/home` partition (or on SD cards), the SteamOS read-only filesystem does not interfere with the downgrading process.

## License
This project is licensed under the MIT License - see the Info dialog in-app for details. Bundled `xdelta3` is licensed under the Apache License 2.0.

## How to Generate Patches

If you want to create patches for a different version of the game, follow these steps:

### Prerequisites
1.  **Source Directory (`SA_STEAM`):** Place the files of the version you want to patch from in a folder named `SA_STEAM`.
2.  **Target Directory (`SA_10US`):** Place the clean v1.0 US (Hoodlum) files in a folder named `SA_10US`.
3.  **xdelta3:** Ensure `xdelta3` is installed on your system.

### Execution

#### Linux
Run the shell script:
```bash
./generate_pathces.sh
```

#### Windows
Run the PowerShell script. If you encounter an execution policy error, use the following command to allow script execution for the current session:
```powershell
Set-ExecutionPolicy -ExecutionPolicy Bypass -Scope Process
.\generate_patches.ps1
```
Alternatively, you can run it directly with the bypass parameter:
```powershell
powershell -ExecutionPolicy Bypass -File .\generate_patches.ps1
```

The scripts will compare the directories, generate `.xdelta` patches in the `Patches/` folder, and create a `manifest.json` file.
