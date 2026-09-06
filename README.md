<div align=center>

<img height="512" alt="GTA SA Open Downgrader logo" src="https://github.com/user-attachments/assets/166a2518-be0a-463b-ad2e-1dec1db89c63" />

# GTA SA Open Downgrader

[![Latest release](https://img.shields.io/github/v/release/xxanqw/gtasa-open-downgrader)](https://github.com/xxanqw/gtasa-open-downgrader/releases/latest)

</div>

GTA SA Open Downgrader is an open-source tool inspired by the original RockstarNexus Downgrader. It converts a supported GTA San Andreas installation to v1.0 US and can install a curated set of community fixes and enhancements.

The current patch set converts the **NewSteam R2 Global** release to **v1.0 US (Hoodlum)**. The app can identify several other executable versions, but identification does not mean that a downgrade patch is available for them.

> [!WARNING]
> I am looking for someone who has the **German NewSteam R2 release** of GTA San Andreas and can help add and verify support for it.
>
> If you can help, please contact me at **[private@xxanqw.pp.ua](mailto:private@xxanqw.pp.ua)**.

Contributions for other source versions are welcome through GitHub issues.

## Patches

- **Source Version:** NewSteam R2 Global
- **Target Version:** v1.0 US (Hoodlum)
- **Download:** [Patches.zip (iCloud)](https://www.icloud.com/iclouddrive/0afGK6zDBog_0drwp6YZoDLIg#Patches)
- **Resolver:** A Go HTTP client resolves the public iCloud file link. Downloads are cancellable; manifests and archive paths are validated before replacing existing patch assets. Local ZIP archives can be imported from Tools.

## Features

- **Game Detection:** Checks the Windows Registry and common Steam locations on Linux. A game directory can always be selected manually.
- **Native Dialogs:** System file/folder pickers, confirmations, messages, and input prompts. Windows needs no extra dialog runtime. Linux uses `qarma` (Qt), `zenity` (GTK), or `matedialog` when installed, with Fyne dialogs as a fallback. Tools and About remain custom Fyne panels.
- **Responsive Startup:** A native Fyne window opens immediately while background preparation runs. No PyInstaller, Qt, Tcl/Tk or splash IPC.
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

When patch assets are missing, the app asks whether it should download them from iCloud. Internet access is also required for mod installation, mod update checks, and application updates.

### Offline

- `gtasa-open-downgrader-windows-offline.exe`
- `gtasa-open-downgrader-linux-offline.AppImage`

Offline builds bundle the downgrade patch assets, so the supported game can be downgraded without an initial patch download. Installing or updating mods still requires internet access.

## Running

### Windows

Download and run the appropriate `.exe` from the [latest GitHub release](https://github.com/xxanqw/gtasa-open-downgrader/releases/latest). No Python, Qt, Tcl/Tk or WebView installation is required.

### Linux (AppImage)

Make the AppImage executable, then launch it:

```bash
chmod +x gtasa-open-downgrader-linux.AppImage
./gtasa-open-downgrader-linux.AppImage
```

AppImages use a statically linked AppImage runtime and do not require `libfuse2`, Python, Qt or Tcl/Tk. Fyne still requires working OpenGL and system X11/Wayland libraries. Release CI builds on Ubuntu 22.04 (glibc 2.35 baseline).
On systems where FUSE is unavailable entirely, use the runtime's extraction fallback:

```bash
./gtasa-open-downgrader-linux.AppImage --appimage-extract-and-run
```

### Steam Deck and Proton

The app checks common native Steam and Flatpak Steam locations plus one common SD-card mount path. If the game is stored elsewhere, select its directory manually. After installing ASI Loader, copy the launch option shown by the app into the game's Steam properties.

## Development

The app is written in Go with [Fyne](https://docs.fyne.io/started/), without HTML or a WebView. Go 1.25 or newer, a C compiler and platform graphics headers are required. Dependencies are pinned in `go.mod` and `go.sum`.

On Ubuntu/Debian:

```bash
sudo apt-get install gcc pkg-config libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev
```

```bash
go run ./cmd/downgrader
go test ./...
go test -race ./...
go vet ./...
./scripts/build.sh online
```

Outputs are in `dist/`. You can provide paths explicitly:

```bash
go run ./cmd/downgrader --game "/path/to/Grand Theft Auto San Andreas" --patches "/path/to/Patches"
```

The app stores downloaded patches in the user's cache directory (`gtasa-open-downgrader/Patches`). It also accepts `Patches` in the working directory or beside the executable. Game backups and the mod tracker remain inside the selected game directory, compatible with earlier releases.

### Windows builds

Install Go and an MSYS2 MinGW-w64 GCC toolchain. Put its `bin` directory on `PATH`, then run from PowerShell:

```powershell
go test ./...
.\scripts\build.ps1 online
```

From Linux with MinGW-w64 installed:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc ./scripts/build.sh online
```

Fyne uses CGO: cross-compilation requires a C compiler for the target, as explained in [Fyne's cross-compilation documentation](https://docs.fyne.io/started/cross-compiling/). Bundled xdelta3 supports Linux/Windows **amd64** only.

### Offline builds and AppImages

Prepare the official patch set once (approximately 880 MB downloaded):

```bash
go run ./cmd/patch-assets
# Or import an archive already downloaded:
go run ./cmd/patch-assets -archive /path/to/Patches.zip
./scripts/build.sh offline
# Windows: .\scripts\build.ps1 offline
```

The offline build embeds the validated patch directory with `go:embed`; a build without `manifest.json` fails. The first run copies embedded patches to a writable cache. Mods and updates still need internet access. Do not commit patch/game assets.

`./scripts/appimage.sh online` and `offline` package built Linux binaries. Set `APPIMAGETOOL` and `APPIMAGE_RUNTIME` to downloaded tools; the release workflow supplies checksum-pinned versions. AppImages bundle liblzma for xdelta3, while graphics libraries/drivers come from the host. The workflow creates online/offline Linux binaries, AppImages, Windows executables and `SHA256SUMS`.

### Package layout and parity

- `cmd/downgrader`: application and update-helper entry point.
- `cmd/patch-assets`: patch download/import/validation for offline builds.
- `internal/game`, `patch`, `mods`, `download`: cancellable domain operations.
- `internal/platform`, `updater`, `jobs`: OS integration, updates and worker lifecycle.
- `internal/ui`, `assets`: Fyne widgets and embedded artwork/binaries.

See [migration parity and compatibility notes](docs/parity.md). The Python implementation is retained in Git history at `cee6363`. ShareMods can challenge automated requests; **Tools → Import mod archive** provides an official-package fallback. Forced mod reinstalls can overwrite customized settings.

## License

This project is licensed under the [MIT License](LICENSE). Bundled `xdelta3` is licensed under the Apache License 2.0; its license is included in `internal/assets/LICENSE.txt` and in the application About dialog.

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
