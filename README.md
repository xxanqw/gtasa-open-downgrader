<div align=center>

<img height="512" alt="GTA SA Open Downgrader logo" src="https://github.com/user-attachments/assets/166a2518-be0a-463b-ad2e-1dec1db89c63" />

# GTA SA Open Downgrader

[![Latest release](https://img.shields.io/github/v/release/xxanqw/gtasa-open-downgrader)](https://github.com/xxanqw/gtasa-open-downgrader/releases/latest)

</div>

GTA SA Open Downgrader is an open-source tool inspired by the original RockstarNexus Downgrader. It converts a supported GTA San Andreas installation to v1.0 US and can install a curated set of community fixes and enhancements.

The current patch set converts the **NewSteam R2 (Global & German)** releases to **v1.0 US (Hoodlum)**. The app can identify several other executable versions, but identification does not mean that a downgrade patch is available for them.

> [!NOTE]
> **Final 0.2.x Release:** This is the last release of the legacy 0.2.x branch and will soon be replaced with a full Go rewrite in the 0.3.x branch.

> [!WARNING]
> Versions prior to v0.2.2 (`<0.2.2`) are discontinued entirely.

Contributions for other source versions are welcome through GitHub issues.

## Patches

- **Source Versions:** NewSteam R2 (Global & German)
- **Target Version:** v1.0 US (Hoodlum)
- **Download:** [Patches_v2.zip](https://files.xxanqw.me/s/6jhSHGdr8N)
- **Direct Download:** Downloads use direct HTTP streaming with server-provided metadata (following redirects and extracting filename/size from headers). The archive is validated and replaces existing patch assets only after successful extraction.

## Features

- **Game Detection:** Checks the Windows Registry and common Steam locations on Linux. A game directory can always be selected manually.
- **Responsive Startup:** Packaged builds show a preparation splash immediately while the one-file application is unpacked and initialized.
- **Version-Aware Scanning:** Identifies known executables and verifies game files against the selected patch manifest.
- **One-Click Downgrade:** Converts supported files to v1.0 US with `xdelta3`, backing up original files before replacing them.
- **Official Mod Packages:** Downloads mods from their official upstream sources and normalizes their archive layouts automatically.
    - ASI Loader & ModLoader
    - SilentPatch & GInput
    - Widescreen Fixes & Project 2DFX
    - SkyGFX
    - Framerate Vigilante (60fps fix, using the current official ZIP package)
    - Frontend Mods
- **Mod Updates:** Records installed package versions and checks upstream releases in the background. The UI marks available updates and can force reinstall selected mods.
- **Legacy Tracker Migration:** Older `.downgrader` files trigger a configuration-backup warning. Every recorded mod is reinstalled once so version tracking can be migrated to the current format.
- **Downgrader Tools:**
    - Apply the 4GB/Large Address Aware flag.
    - Repair the Windows installation path in the Registry.
    - Restore files from downgrade backups and remove old backups.
    - Clear GTA San Andreas user data after confirmation.
    - Create a desktop shortcut on Windows.
- **Windows and Linux Builds:** Includes AppImage builds, common Steam/Flatpak path detection, read-only directory checks, and Proton launch-option guidance for ASI Loader.

## Distribution Variants

Release builds are provided in online and offline variants.

### Online

- `gtasa-open-downgrader-windows.exe`
- `gtasa-open-downgrader-linux.AppImage`

When patch assets are missing, the app asks whether it should download them. Internet access is also required for mod installation, mod update checks, and application updates.

### Offline

- `gtasa-open-downgrader-windows-offline.exe`
- `gtasa-open-downgrader-linux-offline.AppImage`

Offline builds bundle the downgrade patch assets, so the supported game can be downgraded without an initial patch download. Installing or updating mods still requires internet access.

## Running

### Windows

Download and run the appropriate `.exe` from the [latest GitHub release](https://github.com/xxanqw/gtasa-open-downgrader/releases/latest). No Python or Tkinter installation is required.

### Linux (AppImage)

Make the AppImage executable, then launch it:

```bash
chmod +x gtasa-open-downgrader-linux.AppImage
./gtasa-open-downgrader-linux.AppImage
```

Current AppImage builds bundle a statically linked runtime and do not require `libfuse2`, Python, or Tkinter.
On systems where FUSE is unavailable entirely, use the runtime's extraction fallback:

```bash
./gtasa-open-downgrader-linux.AppImage --appimage-extract-and-run
```

### Steam Deck and Proton

The app checks common native Steam and Flatpak Steam locations plus one common SD-card mount path. If the game is stored elsewhere, select its directory manually. After installing ASI Loader, copy the launch option shown by the app into the game's Steam properties.

## Development

Python 3.11 or newer and [`uv`](https://docs.astral.sh/uv/) are required. Release builds use Python 3.12, so it is the recommended development version.

```bash
uv sync --frozen --project downgrader --python 3.12 --group build
uv run --project downgrader python -m unittest discover -s downgrader/tests -v
uv run --project downgrader python downgrader/main.py
```

Running from source uses the Qt splash and does not require Tkinter. Creating a PyInstaller build with `--splash` requires Tcl/Tk on the build machine; PyInstaller embeds those libraries in the resulting executable, so users of release builds do not need to install them.

## License

This project is licensed under the [MIT License](LICENSE). Bundled `xdelta3` is licensed under the Apache License 2.0; its license is included alongside the binary.

## How to Generate Patches

If you want to create patches for a different version of the game, follow these steps:

### Prerequisites

1. **Source Directory (`SA_STEAM`):** Place the files of the version you want to patch from in a folder named `SA_STEAM`.
2. **Target Directory (`SA_10US`):** Place the clean v1.0 US (Hoodlum) files in a folder named `SA_10US`.
3. **xdelta3:** The scripts use the bundled platform binary when available and otherwise expect `xdelta3` on `PATH`.

### Execution

#### Linux

Run the shell script:

```bash
./generate_patches.sh
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
