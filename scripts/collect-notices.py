#!/usr/bin/env python3
"""Retain installed dependency license texts alongside a binary's main license."""
import argparse
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
NAMES = ('license', 'licence', 'copying', 'copyright', 'notice', 'authors')


def collect(directory, label, output):
    if not directory.is_dir():
        raise SystemExit(f'Missing dependency directory: {directory}')
    count = 0
    for current, dirs, files in os.walk(directory):
        dirs[:] = sorted(d for d in dirs if d not in ('.git', 'target', 'build'))
        for name in sorted(files):
            if name.lower().startswith(NAMES):
                path = Path(current) / name
                if path.is_symlink():
                    continue
                output.write(f'\n===== {label}/{path.relative_to(directory).as_posix()} =====\n')
                output.write(path.read_text(encoding='utf-8', errors='replace'))
                output.write('\n')
                count += 1
    return count


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--client', type=Path)
    parser.add_argument('--vcpkg', type=Path)
    args = parser.parse_args()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with args.output.open('w', encoding='utf-8') as output:
        output.write('Third-party notices collected from installed build dependencies.\n'
                     'Each component retains its own license. See docs/LICENSING.md.\n')
        if args.client:
            if not args.vcpkg:
                parser.error('--client requires --vcpkg')
            collect(args.client, 'rustdesk', output)
            cargo = Path(os.environ.get('CARGO_HOME', str(Path.home() / '.cargo')))
            collect(cargo / 'registry' / 'src', 'cargo-registry', output)
            if (cargo / 'git' / 'checkouts').exists():
                collect(cargo / 'git' / 'checkouts', 'cargo-git', output)
            collect(args.vcpkg / 'installed' / 'x64-windows-static' / 'share', 'vcpkg', output)
            # Flutter's generated NOTICES.Z is kept in the bundle's assets as well.
        else:
            subprocess.run(['go', 'mod', 'download'], cwd=ROOT, check=True)
            raw = subprocess.check_output(['go', 'list', '-m', '-json', 'all'], cwd=ROOT, text=True)
            decoder = json.JSONDecoder()
            while raw.strip():
                module, end = decoder.raw_decode(raw.lstrip())
                raw = raw.lstrip()[end:]
                if not module.get('Main') and module.get('Dir'):
                    collect(Path(module['Dir']), module['Path'] + '@' + module['Version'], output)
            collect(ROOT / 'web' / 'node_modules', 'npm', output)


if __name__ == '__main__':
    main()
