#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
variant=${1:-online}
case "$variant" in online|offline) ;; *) echo 'Usage: scripts/build.sh [online|offline]' >&2; exit 2;; esac
system=$(go env GOOS)
if [[ $(go env GOARCH) != amd64 ]]; then echo 'Bundled xdelta3 requires amd64.' >&2; exit 1; fi
name="gtasa-open-downgrader-$system"
tags=release
if [[ "$variant" == offline ]]; then
  # This validation command runs on the build host, including for cross-builds.
  GOOS=$(go env GOHOSTOS) GOARCH=$(go env GOHOSTARCH) CGO_ENABLED=0 go run ./cmd/patch-assets -verify
  tags+=,offline
  name+=-offline
fi
flags="-s -w -X github.com/xxanqw/gtasa-open-downgrader/internal/updater.Version=${VERSION:-v0.2.2}"
if [[ "$system" == windows ]]; then
  name+=.exe
  flags+=' -H=windowsgui'
  "${WINDRES:-x86_64-w64-mingw32-windres}" -i scripts/windows.rc -O coff -o cmd/downgrader/resources_windows_amd64.syso
  trap 'rm -f cmd/downgrader/resources_windows_amd64.syso' EXIT
fi
mkdir -p dist build/tmp
# Offline CGO linking may need several GiB; avoid a small RAM-backed /tmp.
if [[ $(go env GOHOSTOS) == linux ]]; then
  export TMPDIR="$PWD/build/tmp" GOTMPDIR="$PWD/build/tmp"
fi
CGO_ENABLED=1 go build -trimpath -tags "$tags" -ldflags "$flags" -o "dist/$name" ./cmd/downgrader
