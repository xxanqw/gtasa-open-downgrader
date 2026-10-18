#!/usr/bin/env python3
"""Generate/check notices from production Go graphs, not test/go.mod inventories.

Run after dependency changes, review the resulting diff, then commit notices and
manifest together. Native runtime materials in internal/licenses/native need
separate review whenever toolchains/AppImage runtime change.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import zipfile
import io

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / 'internal/licenses'
TARGETS = [('linux', 'linux', 'release', './cmd/downgrader'),
           ('windows', 'windows', 'release', './cmd/downgrader'),
           ('launcher', 'windows', 'release,launcher', './cmd/steam-launcher')]
NESTED = {
    'fyne.io/fyne/v2': ['theme/font/LICENSE.txt', 'theme/font/LICENSE_Inter.txt',
                      'theme/font/LICENSE_DejaVu-Powerline.txt', 'theme/font/LICENSE_EmojiOneColor.txt'],
    'github.com/go-gl/glfw/v3.4/glfw': ['glfw/LICENSE.md'],
    'github.com/fyne-io/image': ['ico/LICENSE-reader', 'ico/LICENSE-writer'],
    'github.com/go-text/typesetting': ['harfbuzz/LICENSE'],
    'github.com/hashicorp/golang-lru/v2': ['simplelru/LICENSE_list'],
    'github.com/andybalholm/brotli': ['flate/LICENSE'],
    'github.com/klauspost/compress': ['internal/snapref/LICENSE', 'zstd/internal/xxhash/LICENSE.txt'],
}
# These are the five existing main-app notices, retained in their original order.
APP_NOTICES = [('App License (MIT)', 'internal/assets/app-LICENSE.txt'),
               ('xdelta3 License', 'internal/assets/LICENSE.txt'),
               ('Font Awesome License', 'internal/assets/fontawesome/LICENSE.txt'),
               ('Hack Font License', 'internal/ui/fonts/LICENSE'),
               ('Noto Sans Font License', 'internal/ui/fonts/LICENSE-NotoSans.txt')]


def json_stream(text):
    decoder = json.JSONDecoder()
    at = 0
    while at < len(text):
        if text[at].isspace():
            at += 1
            continue
        value, at = decoder.raw_decode(text, at)
        yield value


def digest(data):
    return hashlib.sha256(data).hexdigest()


def graph():
    modules, targets = {}, {}
    for name, system, tags, command in TARGETS:
        env = dict(os.environ, GOOS=system, GOARCH='amd64', CGO_ENABLED='1')
        raw = subprocess.check_output(['go', 'list', '-deps', '-json', '-tags', tags, command], cwd=ROOT, env=env, text=True)
        used = set()
        for package in json_stream(raw):
            module = package.get('Module')
            if not module or module.get('Main'):
                continue
            if module.get('Replace'):
                raise RuntimeError('Replaced module needs manual notice review: ' + module['Path'])
            key = module['Path']
            if key in modules and modules[key]['Version'] != module['Version']:
                raise RuntimeError('Conflicting versions for ' + key)
            modules[key] = module
            used.add(key)
        targets[name] = sorted(used)
    return modules, targets


def source_archive(directory):
    # Unmodified module source, including its original MPL/BSD notices and tests.
    # Stable filenames, timestamps and permissions make the archive reproducible.
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for path in sorted(directory.rglob('*')):
            if path.is_symlink():
                raise RuntimeError('Refusing source symlink: ' + str(path))
            if not path.is_file():
                continue
            name = 'golang-lru-v2-v2.0.7/' + path.relative_to(directory).as_posix()
            info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o100644 << 16
            archive.writestr(info, path.read_bytes())
    return buffer.getvalue()


def generate():
    modules, targets = graph()
    sections, records = [], []

    def add(title, path, source):
        data = path.read_bytes().replace(b'\r\n', b'\n')
        text = data.decode('utf-8')
        sections.append(title.upper() + '\n\n' + text)
        records.append({'title': title, 'source': source, 'sha256': digest(data)})

    for title, rel in APP_NOTICES:
        add(title, ROOT / rel, rel)
    sections.append((OUT / 'scope.txt').read_text())
    for key, module in sorted(modules.items()):
        directory = Path(module['Dir'])
        files = sorted(p for p in directory.iterdir() if p.is_file() and
                       re.match(r'^(LICENSE|LICENCE|COPYING|NOTICE|AUTHORS|PATENTS)(?:[._-].*|$)', p.name, re.I))
        if not any(p.name.upper().startswith(('LICENSE', 'LICENCE', 'COPYING')) for p in files):
            raise RuntimeError('Missing root license: ' + key)
        files += [directory / name for name in NESTED.get(key, [])]
        for path in files:
            rel = path.relative_to(directory).as_posix()
            add(key + ' ' + module['Version'] + ' — ' + rel, path, key + '@' + module['Version'] + '/' + rel)
    # Go and native notices are checked in verbatim, not fetched at build time.
    for path in sorted((OUT / 'native').iterdir()):
        if path.is_file():
            add(path.name, path, 'internal/licenses/native/' + path.name)
    module = modules['github.com/hashicorp/golang-lru/v2']
    if module['Version'] != 'v2.0.7':
        raise RuntimeError('Update MPL source archive/version/source URLs before upgrading golang-lru')
    archive = source_archive(Path(module['Dir']))
    manifest = {'schema': 1, 'targets': {key: [{'path': path, 'version': modules[path]['Version']} for path in used]
                                        for key, used in targets.items()},
                'notices': records, 'mpl_source_sha256': digest(archive)}
    text = '\n\n' + '-' * 48 + '\n\n'
    bundle = text.join(sections).encode('utf-8')
    manifest['notices_sha256'] = digest(bundle)
    return {'NOTICES.txt': bundle, 'manifest.json': (json.dumps(manifest, indent=2, ensure_ascii=False) + '\n').encode(),
            'golang-lru-v2-v2.0.7-source.zip': archive}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='fail rather than writing if notice coverage is stale')
    parser.add_argument('--package', type=Path, help='also create a reproducible release notice/source ZIP')
    args = parser.parse_args()
    outputs = generate()
    stale = []
    for name, data in outputs.items():
        path = OUT / name
        if args.check:
            if not path.exists() or path.read_bytes() != data:
                stale.append(name)
        else:
            path.write_bytes(data)
    if stale:
        sys.exit('Stale third-party notices: ' + ', '.join(stale) + '. Run python3 scripts/generate-notices.py and review the diff.')
    if args.package:
        args.package.parent.mkdir(parents=True, exist_ok=True)
        files = dict(outputs)
        files['scope.txt'] = (OUT / 'scope.txt').read_bytes().replace(b'\r\n', b'\n')
        files['native-sources.json'] = (OUT / 'native-sources.json').read_bytes().replace(b'\r\n', b'\n')
        with zipfile.ZipFile(args.package, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
            for name, data in sorted(files.items()):
                info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
                info.compress_type = zipfile.ZIP_DEFLATED
                info.external_attr = 0o100644 << 16
                archive.writestr(info, data)
    print(('Checked' if args.check else 'Generated') + ' production dependency notices and MPL source archive.')


if __name__ == '__main__':
    main()
