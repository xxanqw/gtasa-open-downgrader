# GTA SA Open Downgrader

A Windows and Linux tool for downgrading GTA San Andreas and installing community fixes. Inspired by the original RockstarNexus Downgrader.

**Supported game versions:** NewSteam R2 Global and German → v1.0 US (Hoodlum). Detecting another edition does not mean it can be downgraded.

## Screenshots

<table>
  <tr>
    <td width="50%"><img src="https://ffimg.cc/ed6bb3b4.png" alt="App screenshot 1" width="100%" /></td>
    <td width="50%"><img src="https://ffimg.cc/98da7d56.png" alt="App screenshot 2" width="100%" /></td>
  </tr>
  <tr>
    <td width="50%"><img src="https://ffimg.cc/4f902442.png" alt="App screenshot 3" width="100%" /></td>
    <td width="50%"><img src="https://ffimg.cc/cce81ecd.png" alt="App screenshot 4" width="100%" /></td>
  </tr>
</table>

## Download

Get the app from the [latest release](https://github.com/xxanqw/gtasa-open-downgrader/releases/latest).

| Platform | Online | Offline |
| --- | --- | --- |
| Windows x64 | `gtasa-open-downgrader-windows.exe` | `gtasa-open-downgrader-windows-offline.exe` |
| Linux x64 | `gtasa-open-downgrader-linux.AppImage` | `gtasa-open-downgrader-linux-offline.AppImage` |

The online version downloads patches when needed. The offline version includes them. Downloading mods, multiplayer clients, or updates still requires internet access.

On Windows, run the `.exe`. On Linux:

```bash
chmod +x gtasa-open-downgrader-linux.AppImage
./gtasa-open-downgrader-linux.AppImage
```

If FUSE is unavailable, add `--appimage-extract-and-run`. Linux needs working OpenGL and X11 or Wayland; release builds target Ubuntu 22.04 / glibc 2.35 or newer.

## Downgrade your game

Close GTA and back up your saves and customized mod files before starting.

1. Select your game folder and choose **Scan game**.
2. Download patches when prompted, or import [Patches_v2.1.zip](https://files.xxanqw.me/s/ARC9NX3rm5) through **Tools**. Offline builds already include the patches.
3. Choose **Review downgrade**, check the file changes, and confirm.
4. Let the verification scan finish. **Mods** and **Multiplayer** unlock once the target files are verified.

Original files are saved in `backups/` inside the game folder. Use **Recovery** to restore them. Cancelling stops further work but does not undo completed changes. Restoring downgrade backups does not uninstall mods.

## Mods

The app offers ASI Loader, ModLoader, SilentPatch, Widescreen Fixes, Framerate Vigilante, GInput, SkyGFX, Project 2DFX, and optional interface, font, and translation packages.

Select rows in **Mods**, then use **Install**, **Update**, **Reinstall**, or **Remove**. Required dependencies are selected automatically. Open **Details** for package-specific requirements and caveats, or use the settings gear to edit supported INI files.

Mod removals are backed up under `backups/mods/`. Reinstallation can overwrite customized settings.

ShareMods downloads open in your default browser so you can complete its confirmation and click Download. The app watches your Downloads folder (or a folder you choose), then resumes installation after a new matching archive finishes. Existing/unrelated files and partial downloads are ignored. Reviewed package checksums are still enforced, verified packages are cached for reuse, and your browser download is never deleted. Cancel stops waiting; **Tools → Import mod archive** remains available for existing archives.

## Multiplayer and Steam Deck

Use **Multiplayer** to manage SA-MP, open.mp, MTA, and the optional Steam launcher. The launcher adds a mode chooser to Steam while keeping the real game executable intact. Use **Check updates** to download a verified launcher release, or **Remove & restore** to undo its installation. Close GTA and the launcher before updating. The launcher's **Settings** menu also offers an update check; installation is handled by the downgrader.

On native Windows, installers that require administrator access open a UAC approval prompt; the downgrader itself stays unelevated. Finish or close an elevated installer before cancellation can complete.

On Linux and Steam Deck, clients use the game's detected Steam Proton installation and prefix. Follow the launch options and setup instructions shown in the app. open.mp uses a built-in server browser under Proton.

**Linux multiplayer support is experimental.** Server and anti-cheat compatibility is not guaranteed. MTA 1.6 is the default; MTA 1.7 is an opt-in development build. Keep their installation folders separate.

## Build from source

Requires **Go 1.25+**, an **amd64** target, and a C compiler for Fyne's CGO desktop driver. Run commands from the repository root. Shell build scripts also require Python 3 to validate third-party notices.

### Linux

On Ubuntu/Debian:

```bash
sudo apt-get install gcc pkg-config libgl1-mesa-dev xorg-dev libwayland-dev libxkbcommon-dev liblzma5 python3
go mod download
go run ./cmd/downgrader
```

Build the online binary:

```bash
./scripts/build.sh online
```

### Windows

Install Go and MSYS2's MinGW-w64 GCC toolchain. Put `gcc` and `windres` on `PATH`, then run in PowerShell:

```powershell
$env:CGO_ENABLED = "1"
$env:CC = "gcc"
$env:WINDRES = "windres"
go mod download
go run ./cmd/downgrader
.\scripts\build.ps1 online
```

Build outputs go to `dist/`. Windows builds also produce the Steam launcher, `gta-sa.exe`. For Linux-to-Windows cross-builds and AppImage packaging, see the [release workflow](.github/workflows/build.yml) and [launcher build script](scripts/build-launcher.sh).

### Offline builds

Download and validate the patch assets, then build:

```bash
go run ./cmd/patch-assets
./scripts/build.sh offline
```

On Windows, use `.\scripts\build.ps1 offline` for the second command. To import a local patch ZIP instead, run `go run ./cmd/patch-assets -archive "/path/to/Patches.zip"`. Allow several GiB of free disk space.

To generate patches from your own clean game copies, use [generate_patches.sh](generate_patches.sh) or [generate_patches.ps1](generate_patches.ps1). Do not commit game files or generated patches.

### Release channels

Release builds run only on tag pushes, not branch pushes or manual dispatch:

- `v0.3.0-rc.2`: downgrader prerelease; never becomes Latest or an automatic update.
- `v0.3.0`: stable downgrader release.
- `launcher-1`: launcher-only release, using the independent integer version in `internal/launcherversion/VERSION`. These releases never become the repository's Latest release.

Downgrader releases contain only Windows executables, Linux AppImages, notices, and checksums. Standalone Linux build inputs and `gta-sa.exe` are not published there; the launcher has its own release channel.

Launcher update checks use only `launcher-N` releases; downgrader checks use only stable `vX.Y.Z` releases. `LAUNCHER_VERSION` controls launcher stamping independently of the downgrader's `VERSION`.

### Checks

```bash
go test -tags ci -race ./...
go vet -tags ci ./...
```

The `ci` tag uses Fyne's headless driver; do not use it for desktop release builds.

## Report a problem

[Open an issue](https://github.com/xxanqw/gtasa-open-downgrader/issues) with your OS, game edition, app version, and the log exported from **Log**. Do not upload copyrighted game files.

## License

Original code is licensed under [MIT](LICENSE). Third-party components retain their own terms; see [notices and source availability](internal/licenses/NOTICES.txt) and [distribution qualifications](internal/licenses/scope.txt), also available through **About → Licenses**.
