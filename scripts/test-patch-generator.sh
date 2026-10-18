#!/usr/bin/env bash
# Synthetic fixtures only: no GTA data or generated assets are checked in.
set -euo pipefail
repo=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/internal/assets" "$work/scripts" "$work/SA_STEAM/models" "$work/SA_STEAM_GERMAN/models" "$work/SA_10US/models"
cp "$repo/scripts/verify-german-overlay.py" "$work/scripts/"
cp "$repo/generate_patches.sh" "$work/"
cp "$repo/internal/assets/xdelta3_linux" "$work/internal/assets/"
printf global > "$work/SA_STEAM/gta-sa.exe"
printf german > "$work/SA_STEAM_GERMAN/gta-sa.exe"
printf target > "$work/SA_10US/gta_sa.exe"
printf 'global only' > "$work/SA_STEAM/models/global.img"
printf 'german only' > "$work/SA_STEAM_GERMAN/models/german.img"
printf 'target global' > "$work/SA_10US/models/global.img"
printf 'target german' > "$work/SA_10US/models/german.img"
if ! (cd "$work" && bash generate_patches.sh > run.log 2>&1); then
    printf 'Generator failed:\n' >&2
    grep -v 'Progress:' "$work/run.log" >&2 || true
    exit 1
fi
python3 - "$work/Patches/manifest.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1]))
assert m['version'] == '2.1' and m['release_id'] == 'steam-r2-global-german-v2-complete'
by_path = {f['path']: f for f in m['files']}
assert by_path['models/global.img']['applicable_versions'] == ['steam_r2_global']
assert by_path['models/german.img']['applicable_versions'] == ['steam_r2_german']
assert set(by_path['gta-sa.exe']['applicable_versions']) == {'steam_r2_global', 'steam_r2_german'}
PY
(cd "$repo" && go run ./cmd/patch-assets -target "$work/Patches" -verify -release > /dev/null)
# A German-only executable change still needs a copy entry when Global is already target.
printf target > "$work/SA_STEAM/gta-sa.exe"
(cd "$work" && bash generate_patches.sh > run.log 2>&1)
(cd "$repo" && go run ./cmd/patch-assets -target "$work/Patches" -verify -release > /dev/null)
python3 - "$work/Patches/manifest.json" <<'PY'
import json, sys
m = json.load(open(sys.argv[1]))
assert len([f for f in m['files'] if f['action'] == 'copy']) == 1
PY
# Missing target files must fail rather than producing a partial release.
printf extra > "$work/SA_STEAM_GERMAN/models/extra.img"
if (cd "$work" && bash generate_patches.sh > run.log 2>&1); then
    echo 'generator accepted a German file absent from target' >&2
    exit 1
fi
grep -q 'Refusing to publish a partial manifest' "$work/run.log"
test ! -e "$work/Patches/manifest.json"
# A verified sparse difference overlay inherits unchanged Global files.
rm -f "$work/SA_STEAM/models/global.img" "$work/SA_STEAM_GERMAN/models/german.img" "$work/SA_STEAM_GERMAN/models/extra.img" "$work/SA_10US/models/global.img" "$work/SA_10US/models/german.img"
printf 'shared Global/German source' > "$work/SA_STEAM/models/shared.img"
printf 'shared target' > "$work/SA_10US/models/shared.img"
python3 - "$work" <<'PY'
import hashlib, json, pathlib, sys
work = pathlib.Path(sys.argv[1])
def info(path):
    data = path.read_bytes()
    return {'size': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
report = {
    'reference_version': 'steam_r2_global', 'scanned_version': 'steam_r2_german',
    'summary': {'global_files': 2, 'german_files': 2, 'identical': 1, 'different': 1, 'missing': 0, 'extra': 0, 'errors': 0},
    'different': [{'path': 'gta-sa.exe', 'global_path': 'gta-sa.exe', 'global': info(work / 'SA_STEAM/gta-sa.exe'), 'german': info(work / 'SA_STEAM_GERMAN/gta-sa.exe')}],
}
(work / 'SA_STEAM_GERMAN/german_r2_diff.json').write_text(json.dumps(report))
PY
(cd "$work" && bash generate_patches.sh > run.log 2>&1)
(cd "$repo" && go run ./cmd/patch-assets -target "$work/Patches" -verify -release > /dev/null)
python3 - "$work/Patches/manifest.json" <<'PY'
import json, sys
files = {f['path']: f for f in json.load(open(sys.argv[1]))['files']}
assert set(files['models/shared.img']['applicable_versions']) == {'steam_r2_global', 'steam_r2_german'}
PY
printf tampered > "$work/SA_STEAM_GERMAN/gta-sa.exe"
if (cd "$work" && bash generate_patches.sh > run.log 2>&1); then
    echo 'generator accepted an unverified German overlay' >&2
    exit 1
fi
test ! -e "$work/Patches/manifest.json"
echo 'Synthetic generator fixtures passed'
