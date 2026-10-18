#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
python3 scripts/generate-notices.py --check --package dist/THIRD-PARTY-NOTICES.zip
variant=${1:-online}
case "$variant" in online|offline) ;; *) echo 'Usage: scripts/build.sh [online|offline]' >&2; exit 2;; esac
system=$(go env GOOS)
launcher_version=${LAUNCHER_VERSION:-$(tr -d '\r\n' < internal/launcherversion/VERSION)}
if [[ ! "$launcher_version" =~ ^[1-9][0-9]{0,19}$ ]]; then echo 'Invalid launcher version; use a positive release number.' >&2; exit 2; fi
if [[ $(go env GOARCH) != amd64 ]]; then echo 'Bundled xdelta3 requires amd64.' >&2; exit 1; fi
name="gtasa-open-downgrader-$system"
tags=release
if [[ "$variant" == offline ]]; then
  # This validation command runs on the build host, including for cross-builds.
  GOOS=$(go env GOHOSTOS) GOARCH=$(go env GOHOSTARCH) CGO_ENABLED=0 go run ./cmd/patch-assets -verify -release
  tags+=,offline
  name+=-offline
fi
flags="-s -w -X github.com/xxanqw/gtasa-open-downgrader/internal/updater.Version=${VERSION:-v0.3.0-rc.1}"
if [[ "$system" == windows ]]; then
	name+=.exe
	flags+=' -H=windowsgui'
	"${WINDRES:-x86_64-w64-mingw32-windres}" -i scripts/windows.rc -O coff -o cmd/downgrader/resources_windows_amd64.syso
	"${WINDRES:-x86_64-w64-mingw32-windres}" -i scripts/windows-launcher.rc -O coff -o cmd/steam-launcher/resources_windows_amd64.syso
	trap 'rm -f cmd/downgrader/resources_windows_amd64.syso cmd/steam-launcher/resources_windows_amd64.syso' EXIT
fi
mkdir -p dist build/tmp
# Offline CGO linking may need several GiB; avoid a small RAM-backed /tmp.
if [[ $(go env GOHOSTOS) == linux ]]; then
  export TMPDIR="$PWD/build/tmp" GOTMPDIR="$PWD/build/tmp"
fi
CGO_ENABLED=1 go build -trimpath -tags "$tags" -ldflags "$flags" -o "dist/$name" ./cmd/downgrader
if [[ "$system" == windows ]]; then
	CGO_ENABLED=1 go build -trimpath -tags release,launcher -ldflags "-s -w -H=windowsgui -X github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion.Identity=GTASA_STEAM_LAUNCHER_VERSION[$launcher_version]" -o dist/gta-sa.exe ./cmd/steam-launcher
fi
