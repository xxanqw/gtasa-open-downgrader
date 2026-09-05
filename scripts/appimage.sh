#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
variant=${1:-online}
case "$variant" in online) name=gtasa-open-downgrader-linux;; offline) name=gtasa-open-downgrader-linux-offline;; *) echo 'Use online or offline' >&2; exit 2;; esac
: "${APPIMAGETOOL:?Set APPIMAGETOOL to appimagetool executable}"
: "${APPIMAGE_RUNTIME:?Set APPIMAGE_RUNTIME to the static type2 runtime}"
appdir=$(mktemp -d)
trap 'rm -rf "$appdir"' EXIT
mkdir -p "$appdir/usr/bin" "$appdir/usr/lib" "$appdir/usr/share/licenses/gtasa-open-downgrader"
cp "dist/$name" "$appdir/usr/bin/$name"
cp LICENSE internal/assets/LICENSE.txt "$appdir/usr/share/licenses/gtasa-open-downgrader/"
cp internal/assets/icon.png "$appdir/gtasa-open-downgrader.png"
# xdelta3 needs liblzma when extracted at runtime. Keep graphics drivers and
# glibc supplied by the host; build on Ubuntu 22.04 for a conservative baseline.
lzma=$(ldd internal/assets/xdelta3_linux | awk '/liblzma.so/ {print $3}')
[[ -n "$lzma" && -f "$lzma" ]] || { echo 'liblzma not found' >&2; exit 1; }
cp "$lzma" "$appdir/usr/lib/liblzma.so.5"
cat > "$appdir/gtasa-open-downgrader.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=GTA SA Open Downgrader
Exec=$name
Icon=gtasa-open-downgrader
Categories=Game;Utility;
DESKTOP
cat > "$appdir/AppRun" <<'APPRUN'
#!/bin/sh
HERE=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
export LD_LIBRARY_PATH="$HERE/usr/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
APPRUN
printf 'exec "$HERE/usr/bin/%s" "$@"\n' "$name" >> "$appdir/AppRun"
chmod +x "$appdir/AppRun"
ln -s gtasa-open-downgrader.png "$appdir/.DirIcon"
APPIMAGE_EXTRACT_AND_RUN=1 ARCH=x86_64 "$APPIMAGETOOL" --runtime-file "$APPIMAGE_RUNTIME" "$appdir" "dist/$name.AppImage"
