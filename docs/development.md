# Development

[Back to the project](../README.md)

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

Build outputs go to `dist/`. Windows builds also produce the Steam launcher, `gta-sa.exe`. For Linux-to-Windows cross-builds and AppImage packaging, see the [release workflow](../.github/workflows/build.yml) and [launcher build script](../scripts/build-launcher.sh).

### Offline builds

Download and validate the patch assets, then build:

```bash
go run ./cmd/patch-assets
./scripts/build.sh offline
```

On Windows, use `.\scripts\build.ps1 offline` for the second command. To import a local patch ZIP instead, run `go run ./cmd/patch-assets -archive "/path/to/Patches.zip"`. Allow several GiB of free disk space.

To generate patches from your own clean game copies, use [generate_patches.sh](../generate_patches.sh) or [generate_patches.ps1](../generate_patches.ps1). Do not commit game files or generated patches.

### Checks

```bash
go test -tags ci -race ./...
go vet -tags ci ./...
```

The `ci` tag uses Fyne's headless driver; do not use it for desktop release builds.

Actions caches only Go module downloads. Compiled objects, including embedded offline patch data, stay on the temporary runner. Only default-branch runs save the shared module cache; release tags and pull requests restore it without creating per-release copies.

See [Publishing releases](releasing.md) for versioning and release checks.
