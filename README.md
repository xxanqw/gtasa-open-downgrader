<div align=center>

<img height="512" alt="GTA SA Open Downgrader logo" src="https://github.com/user-attachments/assets/e4177c99-aa80-4b15-8615-7b0056497821" />

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
- **Font Awesome Icons:** Embedded Font Awesome Free SVG icons follow the monochrome theme across the toolbar, controls, menus, and file dialogs. No icon font or network connection is needed. Attribution and license information are included in About.
- **Monochrome Interface:** A compact grayscale theme follows the system light/dark preference. File statuses stay explicit, with bold text for missing, modified, or failed files. Dialogs share the same compact monochrome styling.
- **Dialogs:** File, folder, and export destinations use the operating system's file explorer, with a Fyne fallback when it is unavailable. Confirmations, messages, Tools, and About keep the compact monochrome Fyne design.
- **Application Log:** The footer’s ⓘ button opens a live session journal with operation events, Fyne diagnostics, and error details. Copy or export the retained log (up to 1 MiB) as a `.log` file. Worker panics are reported with stack traces; fatal runtime crash reports are appended to `gtasa-open-downgrader/last-crash.log` in the system user cache directory.
- **Responsive Startup:** A native Fyne window opens immediately while background preparation runs. No PyInstaller, Qt, Tcl/Tk or splash IPC.
- **Version-Aware Scanning:** Identifies known executables and verifies game files against the selected patch manifest.
- **One-Click Downgrade:** Converts supported files to v1.0 US with `xdelta3`, backing up original files before replacing them.
- **Curated Mod Packages:** Downloads mods from their upstream sources or a documented manual-package mirror and normalizes their archive layouts automatically.
    - ASI Loader & ModLoader
    - SilentPatch & GInput
    - Widescreen Fixes & Project 2DFX
    - SkyGFX
    - Framerate Vigilante (60fps fix, using the current official ZIP package)
    - Frontend Mods
    - SALtd Russian (SanLtd Team 0.56, installed from the manual ZIP without its executable or intro videos)
- **Mod Selection:** An always-visible compact panel above the file table lets you choose mods, check installed versions, and install or reinstall selected packages.
- **Mod Updates:** Records installed package versions and checks upstream releases in the background. The UI marks available updates and can force reinstall selected mods.
- **Legacy Tracker Migration:** Older `.downgrader` files trigger a configuration-backup warning. Every recorded mod is reinstalled once so version tracking can be migrated to the current format.
- **Downgrader Tools:**
    - Apply the 4GB/Large Address Aware flag.
    - Repair the Windows installation path in the Registry.
    - Restore files from downgrade backups and remove old backups.
    - Clear GTA San Andreas user data after confirmation.
    - Create a desktop shortcut on Windows.
- **Windows and Linux Builds:** Includes AppImage builds, common Steam/Flatpak path detection, read-only directory checks, and Proton launch-option guidance for ASI Loader.
- **Steam Launcher:** Installs a compact Windows Fyne menu as Steam's `gta-sa.exe` while keeping the real downgraded game as `gta_sa.exe`. The operation preserves an existing entrypoint, verifies launcher ownership, supports safe removal, and detects Steam file verification replacing it.
- **Multiplayer Management:** Detects SA-MP, open.mp, and Multi Theft Auto and provides install, repair, and uninstall actions from a compact Multiplayer dialog. Linux runs compatible clients in GTA SA's detected Steam Proton context; MTA remains available on native Windows but is currently disabled under Wine/Proton because its upstream integrity system terminates the client.

## Distribution Variants

Release builds are provided in online and offline variants.

### Online

- `gtasa-open-downgrader-windows.exe`
- `gtasa-open-downgrader-linux.AppImage`
- `gta-sa.exe` (Windows Steam launcher, also bundled inside the AppImage)

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

Open **Multiplayer** after scanning the game to install or repair the Steam launcher and multiplayer clients. The launcher replaces Steam's `gta-sa.exe` entrypoint, preserves the previous file, and always starts the real game from `gta_sa.exe`. **Remove & restore** returns the preserved entrypoint without deleting the real game. If Steam file verification restores `gta-sa.exe`, the dialog reports the launcher as replaced and offers to reinstall it.

SA-MP requires the user to download its installer from the official successor page because there is no stable first-party artifact URL. open.mp and MTA download from their official sources. Their normal Windows installer windows appear through the game's selected Proton version and prefix; the app never calls system Wine or protontricks. Hidden `.steam` directories do not need to be browsed manually: the game receives a visible path such as `G:\Grand Theft Auto San Andreas`, the SA-MP destination is prefilled, and the GTA path is registered for MTA. MTA keeps its normal separate installation directory. Because open.mp's WebView interface does not run under Wine, its Steam launcher button opens a compact connection form under Proton with the full open.mp-managed SA-MP client list, including R1 through R5 and 0.3.DL. See [the architecture and current upstream assumptions](docs/steam-launcher-multiplayer.md) for client-specific details and limitations.

Installed clients can be repaired or removed from the same dialog. Removal opens each client's standard uninstaller in the detected Windows or Proton environment. The open.mp action also removes the managed command-line client files added by the downgrader. On Windows, SA-MP font registrations are released before cleanup; any font file still held by another process is scheduled for deletion at the next restart.

## Development

The app uses Go and Fyne, without HTML or a WebView. Use **Go 1.25 or newer** (see `go.mod`) and an **amd64** Linux or Windows build environment. Fyne's desktop driver uses CGO, so running or building the GUI requires a C compiler and platform graphics dependencies. Dependencies are pinned in `go.mod` and `go.sum`; Python and the Fyne CLI are not required for normal development or builds.

Run the following commands from the repository root.

### Linux setup and local runs

On Ubuntu/Debian, install the compiler, graphics headers and xdelta3 runtime dependency:

```bash
sudo apt-get update
sudo apt-get install gcc pkg-config libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev liblzma5
go mod download
go run ./cmd/downgrader
```

To start with a specific game folder and an existing patch directory:

```bash
go run ./cmd/downgrader --game "/path/to/Grand Theft Auto San Andreas" --patches "/path/to/Patches"
```

A normal development run uses the online variant and does not require bundled patches. The app stores downloaded patches in the system user cache directory under `gtasa-open-downgrader/Patches`. It also accepts `Patches` in the working directory or beside the executable. Game backups and the mod tracker remain inside the selected game directory, compatible with earlier releases.

### Formatting, tests and UI previews

Run the same checks as [CI](.github/workflows/test.yml):

```bash
gofmt -w cmd internal
go test -race ./...
go vet ./...
./scripts/build.sh online
```

For tests on a machine without a graphical desktop or graphics development libraries, use Fyne's software driver:

```bash
go test -tags ci ./...
go vet -tags ci ./...
```

The `ci` tag is for headless checks, not distributable desktop builds. Race testing still requires CGO and a C compiler.

UI tests can export PNG previews without opening a desktop window. For example:

```bash
mkdir -p build/previews
GTASA_COMPACT_SNAPSHOT="$PWD/build/previews/main.png" \
GTASA_PROGRESS_SNAPSHOT="$PWD/build/previews/progress.png" \
go test -tags ci -count=1 -run '^TestCompactWindowKeepsGameInformationVisible$' ./internal/ui
```

### Linux release builds

```bash
./scripts/build.sh online
./dist/gtasa-open-downgrader-linux --version
```

The scripts enable CGO, apply the `release` build tag, strip debugging symbols and write binaries to `dist/`. Set `VERSION` to override the embedded application version:

```bash
VERSION=v0.2.2 ./scripts/build.sh online
```

Online and offline builds produce `gtasa-open-downgrader-linux` and `gtasa-open-downgrader-linux-offline`, respectively. These binaries still depend on host graphics libraries. For portable releases, use the Ubuntu 22.04 build environment from CI to retain its glibc 2.35 baseline; building on a newer distribution can increase the required glibc version.

### Windows development and builds

Install Go and an MSYS2 MinGW-w64 GCC toolchain, including `gcc` and `windres`. Add the toolchain's `bin` directory (for example, `C:\msys64\mingw64\bin`) to `PATH`. From PowerShell:

```powershell
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:WINDRES = "windres"
go mod download
go run ./cmd/downgrader
go test ./...
go vet ./...
.\scripts\build.ps1 online
```

The outputs are `dist/gtasa-open-downgrader-windows.exe` and the Steam launcher `dist/gta-sa.exe`. Set `$env:VERSION` to override the embedded downgrader version. The build script embeds the Windows icon and GUI manifest into both executables and removes its temporary resource files afterwards.

To cross-build from Linux, install MinGW-w64 and run:

```bash
GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
WINDRES=x86_64-w64-mingw32-windres ./scripts/build.sh online
```

To build only the Windows Steam launcher from Linux:

```bash
sudo apt-get install mingw-w64
./scripts/build-launcher.sh
```

The result is `dist/gta-sa.exe`. The `-H=windowsgui` linker flag prevents a console window. Linux AppImage packaging expects this file and includes it under `usr/libexec`, where the downgrader can install it into the selected game directory.

Changing `GOOS` alone is insufficient: Fyne needs a target C compiler. Bundled xdelta3 supports Linux/Windows **amd64** only. Check the resulting GUI on Windows as well; a successful cross-build does not exercise Windows behavior.

### Offline patches and builds

Prepare the official patch set once (approximately 880 MB downloaded):

```bash
go run ./cmd/patch-assets
```

Alternatively, import an existing archive:

```bash
go run ./cmd/patch-assets -archive "/path/to/Patches.zip"
```

Both commands validate the assets and place them in `internal/assets/Patches`. To verify an existing copy without downloading anything:

```bash
go run ./cmd/patch-assets -verify
```

Then build the offline variant:

```bash
./scripts/build.sh offline
```

On Windows, use `.\scripts\build.ps1 offline`; for a local offline development run, use `go run -tags offline ./cmd/downgrader`.

Offline builds embed the validated directory with `go:embed`. The build scripts revalidate it; missing `manifest.json` prevents compilation. The first run copies embedded patches to a writable cache. Allow several GiB of free disk space for offline compilation and packaging; the Linux build script uses `build/tmp` for temporary files to avoid small RAM-backed `/tmp` mounts. Mods and update checks still need internet access. The asset preparation and build directories are ignored by Git. Do not commit patch/game assets or game backups.

### AppImage packaging and release CI

Build the Linux binaries first. Download the checksum-pinned `appimagetool` and static type2 runtime specified in [the release workflow](.github/workflows/build.yml), verify their checksums, then supply their absolute paths:

```bash
chmod +x /path/to/appimagetool
export APPIMAGETOOL=/path/to/appimagetool
export APPIMAGE_RUNTIME=/path/to/runtime-x86_64
./scripts/appimage.sh online
./scripts/appimage.sh offline
```

Each command requires the corresponding binary already in `dist/` and writes its `.AppImage` beside it. AppImages include liblzma for xdelta3 and the application, xdelta3 and Font Awesome licenses. Graphics libraries and drivers come from the host.

The release workflow runs on `v*` tags or a manual workflow dispatch. It builds both variants for Linux and Windows and packages AppImages. **Only a version tag publishes a GitHub release**, including `SHA256SUMS`; a manual run uploads workflow artifacts. Ordinary branch pushes run the checks workflow.

### Package layout and asset maintenance

- `cmd/downgrader`: application and update-helper entry point.
- `cmd/steam-launcher`: compact, network-free Windows Steam menu built as `gta-sa.exe`.
- `cmd/multiplayer-context`: developer diagnostic for launcher and client discovery.
- `cmd/patch-assets`: patch download/import/validation for offline builds.
- `internal/entrypoint`: launcher identity, executable selection, install, backup and restore.
- `internal/multiplayer`, `launcherapp`: client discovery/install execution and launcher runtime.
- `internal/game`, `patch`, `mods`, `download`: cancellable domain operations using `context.Context`.
- `internal/platform`, `updater`, `jobs`: OS integration, updates and worker lifecycle.
- `internal/steam`: Steam libraries, AppID manifests, compatibility-tool resolution and reusable Proton execution.
- `internal/applog`: bounded application journal, error reporting and log export.
- `internal/ui`: Fyne windows, dialogs, themes, layouts and UI tests.
- `internal/assets`: embedded artwork, Font Awesome SVGs, licenses, xdelta3 binaries and optional offline patches.

When replacing Font Awesome SVGs, preserve their attribution and run `scripts/normalize-icons.py` with Python and `fonttools==4.64.0`. This maintenance step converts problematic SVG arcs and makes checkbox canvases square for Fyne's renderer. Commit the normalized SVGs; end users and normal Go builds do not need Python. See [icon attribution and maintenance notes](internal/assets/fontawesome/README.md).

See [migration parity and compatibility notes](docs/parity.md), the [Steam/Proton integration notes](docs/steam-proton.md), and the [Steam launcher and multiplayer architecture](docs/steam-launcher-multiplayer.md). The Python implementation is retained in Git history at `cee6363`. ShareMods can challenge automated requests; **Tools → Import mod archive** provides an official-package fallback. Forced mod reinstalls can overwrite customized settings.

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
