#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
python3 scripts/generate-notices.py --check --package dist/THIRD-PARTY-NOTICES.zip
if [[ $(go env GOARCH) != amd64 ]]; then echo 'The Steam launcher currently targets Windows amd64.' >&2; exit 1; fi
version=${LAUNCHER_VERSION:-$(tr -d '\r\n' < internal/launcherversion/VERSION)}
if [[ ! "$version" =~ ^[1-9][0-9]{0,19}$ ]]; then echo 'Invalid launcher version; use a positive release number.' >&2; exit 2; fi
windres=${WINDRES:-x86_64-w64-mingw32-windres}
compiler=${CC:-x86_64-w64-mingw32-gcc}
resource=cmd/steam-launcher/resources_windows_amd64.syso
trap 'rm -f "$resource"' EXIT
"$windres" -i scripts/windows-launcher.rc -O coff -o "$resource"
mkdir -p dist build/tmp
if [[ $(go env GOHOSTOS) == linux ]]; then
  export TMPDIR="$PWD/build/tmp" GOTMPDIR="$PWD/build/tmp"
fi
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC="$compiler" go build -trimpath -tags release,launcher -ldflags "-s -w -H=windowsgui -X github.com/xxanqw/gtasa-open-downgrader/internal/launcherversion.Identity=GTASA_STEAM_LAUNCHER_VERSION[$version]" -o dist/gta-sa.exe ./cmd/steam-launcher
