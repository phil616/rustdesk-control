#!/usr/bin/env python3
"""Publish only the two supported executables; never overwrite a published release."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile

ASSETS = ('rustdesk-control-linux-amd64', 'rustdesk-managed-windows-amd64.exe')


def gh(*args):
    return subprocess.check_output(['gh', *args], text=True)


def main():
    directory = Path('dist/release')
    if sorted(p.name for p in directory.iterdir()) != sorted(ASSETS):
        raise SystemExit('Release directory must contain exactly the two supported binaries')
    tag, commit = os.environ['RELEASE_TAG'], os.environ['RELEASE_COMMIT']
    repository = os.environ['GH_REPO']
    # List first: unlike treating every `gh release view` failure as 404, network
    # or authentication failures cannot accidentally start a new release here.
    releases = json.loads(gh('api', '--paginate', '--slurp', f'repos/{repository}/releases?per_page=100'))
    existing = next((r for page in releases for r in page if r['tag_name'] == tag), None)
    if existing and not existing['draft']:
        raise SystemExit('Release already published; create a new tag instead of replacing binaries')
    if existing and any(a['name'] not in ASSETS for a in existing['assets']):
        raise SystemExit('Existing draft contains unexpected assets; review it before retrying')
    hashes = '\n'.join(f'{hashlib.sha256((directory / name).read_bytes()).hexdigest()}  {name}' for name in ASSETS)
    notes = f'''Two supported targets only:
- `rustdesk-control-linux-amd64`: Go control plane with embedded Web UI.
- `rustdesk-managed-windows-amd64.exe`: Windows x64 modified RustDesk client (upstream 1.4.9).

The Windows EXE is unsigned; use the normal installer and retain UAC prompts.
Build success does not certify real-device remote login or reboot acceptance.

License: AGPL-3.0-only for project changes; upstream/dependency notices retained.
[Deployment and build documentation](https://github.com/{repository}/tree/{commit}/docs)
[Corresponding source and license instructions](https://github.com/{repository}/blob/{commit}/docs/LICENSING.md)
The control plane serves `/source.tar.gz` and `/LICENSE`. The Windows EXE payload
contains `corresponding-source.tar.gz`, `SOURCE-CODE.md`, and license notices.
GitHub's automatic repository source snapshots contain the control project and
patches; they do not by themselves contain the expanded RustDesk checkout.

Commit: `{commit}`

SHA-256:
```
{hashes}
```
'''
    with tempfile.TemporaryDirectory() as tmp:
        body = Path(tmp) / 'release.md'
        body.write_text(notes, encoding='utf-8')
        if not existing:
            args = ['release', 'create', tag, '--verify-tag', '--draft', '--title', tag, '--notes-file', str(body)]
            if '-' in tag:
                args.append('--prerelease')
            gh(*args)
        else:
            gh('release', 'edit', tag, '--notes-file', str(body))
        gh('release', 'upload', tag, *(str(directory / name) for name in ASSETS), '--clobber')
        actual = json.loads(gh('release', 'view', tag, '--json', 'assets'))['assets']
        if sorted(a['name'] for a in actual) != sorted(ASSETS):
            raise SystemExit('Asset validation failed; release remains a draft')
        gh('release', 'edit', tag, '--draft=false')


if __name__ == '__main__':
    main()
