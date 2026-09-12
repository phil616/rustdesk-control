#!/usr/bin/env python3
"""Package editable project sources (including modified upstream/submodules).

Only Git-selected source files are collected; ignored credentials, build outputs,
.git metadata and databases are never traversed. No Git archive is used for the
client: it would discard the modifications applied on top of the upstream tag.
"""
import argparse
import gzip
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile

ROOT = Path(__file__).resolve().parents[1]


def git(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args])


def add_tree(archive, repo, prefix):
    # Include untracked non-ignored source additions in local development builds.
    tracked = set(git(repo, 'ls-files', '--cached', '-z').split(b'\0'))
    untracked = set(git(repo, 'ls-files', '--others', '--exclude-standard', '-z').split(b'\0'))
    build_dirs = {'target', 'build', 'dist', 'node_modules', '__pycache__', '.dart_tool'}
    untracked = {p for p in untracked if not build_dirs.intersection(Path(os.fsdecode(p)).parts)}
    paths = sorted(tracked | untracked)
    for raw in paths:
        if not raw:
            continue
        relative = Path(os.fsdecode(raw))
        path = repo / relative
        if path.is_symlink():
            # Preserve link text, never dereference an external file.
            archive.add(path, arcname=str(Path(prefix) / relative), recursive=False)
        elif path.is_file():
            archive.add(path, arcname=str(Path(prefix) / relative), recursive=False)
    # Gitlinks are directories and skipped above; include their actual source.
    submodules = repo / '.gitmodules'
    if submodules.exists():
        result = subprocess.run(['git', '-C', str(repo), 'config', '-f', '.gitmodules', '--get-regexp', r'^submodule\..*\.path$'], capture_output=True, text=True)
        if result.returncode not in (0, 1):
            raise RuntimeError('Unable to enumerate submodules')
        for line in result.stdout.splitlines():
            relative = Path(line.split(' ', 1)[1])
            path = repo / relative
            if not (path / '.git').exists():
                raise RuntimeError(f'Uninitialized source submodule: {path}')
            add_tree(archive, path, str(Path(prefix) / relative))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--client', type=Path)
    args = parser.parse_args()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    # Stage outside the checkout so an untracked output cannot include itself.
    import tempfile
    with tempfile.TemporaryFile() as staging:
        with gzip.GzipFile(fileobj=staging, mode='wb', mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode='w') as archive:
                add_tree(archive, ROOT, 'rustdesk-control')
                if args.client:
                    client = args.client.resolve()
                    add_tree(archive, client, 'rustdesk-managed-client')
                    # Upstream ignores its generated bridge. Include exactly the generated source.
                    for name in ['src/bridge_generated.rs', 'src/bridge_generated.io.rs', 'flutter/lib/generated_bridge.dart', 'flutter/lib/generated_bridge.freezed.dart']:
                        path = client / name
                        if path.is_file():
                            archive.add(path, arcname='rustdesk-managed-client/' + name, recursive=False)
                metadata = {'control_commit': git(ROOT, 'rev-parse', 'HEAD').decode().strip(),
                            'control_url': os.getenv('RUSTDESK_CONTROL_URL', ''),
                            'source_url': os.getenv('RUSTDESK_MANAGED_SOURCE_URL', ''),
                            'client_base': '6c578292e8ebbbec708b76986ba8c4bc7c509747' if args.client else None}
                data = json.dumps(metadata, indent=2).encode()
                info = tarfile.TarInfo('BUILD-SOURCE.json')
                info.size = len(data)
                archive.addfile(info, io.BytesIO(data))
        staging.seek(0)
        with args.output.open('wb') as output:
            import shutil
            shutil.copyfileobj(staging, output)
    print(f'Source archive: {args.output}')


if __name__ == '__main__':
    main()
