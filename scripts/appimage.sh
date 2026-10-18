#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
variant=${1:-online}
case "$variant" in online) name=gtasa-open-downgrader-linux;; offline) name=gtasa-open-downgrader-linux-offline;; *) echo 'Use online or offline' >&2; exit 2;; esac
: "${APPIMAGETOOL:?Set APPIMAGETOOL to appimagetool executable}"
: "${APPIMAGE_RUNTIME:?Set APPIMAGE_RUNTIME to the static type2 runtime}"
mkdir -p build
appdir=$(mktemp -d "$PWD/build/AppDir.XXXXXX")
trap 'rm -rf "$appdir"' EXIT
[[ -f dist/gta-sa.exe ]] || { echo 'Build dist/gta-sa.exe with scripts/build-launcher.sh first.' >&2; exit 1; }
mkdir -p "$appdir/usr/bin" "$appdir/usr/lib" "$appdir/usr/libexec" "$appdir/usr/share/licenses/gtasa-open-downgrader"
cp "dist/$name" "$appdir/usr/bin/$name"
cp dist/gta-sa.exe "$appdir/usr/libexec/gta-sa.exe"
python3 scripts/generate-notices.py --check --package dist/THIRD-PARTY-NOTICES.zip
cp internal/licenses/NOTICES.txt internal/licenses/manifest.json internal/licenses/native-sources.json internal/licenses/scope.txt internal/licenses/golang-lru-v2-v2.0.7-source.zip "$appdir/usr/share/licenses/gtasa-open-downgrader/"
# Notices are not a substitute for the pinned static runtime's LGPL source and
# relinking materials. Verify those separately before publishing an AppImage.
cp internal/assets/icon.png "$appdir/gtasa-open-downgrader.png"
# xdelta3 needs liblzma when extracted at runtime. Keep graphics drivers and
# glibc supplied by the host; build on Ubuntu 22.04 for a conservative baseline.
lzma=$(ldd internal/assets/xdelta3_linux | awk '/liblzma.so/ {print $3}')
[[ -n "$lzma" && -f "$lzma" ]] || { echo 'liblzma not found' >&2; exit 1; }
cp "$lzma" "$appdir/usr/lib/liblzma.so.5"
cat > "$appdir/me.xxanqw.gtasa-open-downgrader.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=GTA SA Open Downgrader
Exec=$name
Icon=gtasa-open-downgrader
Categories=Game;
StartupWMClass=me.xxanqw.gtasa-open-downgrader
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
