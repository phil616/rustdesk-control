#!/usr/bin/env python3
"""Exercise managed release URL fail-closed checks without native GUI dependencies."""
import os
from pathlib import Path
import subprocess
import tempfile

root = Path(__file__).resolve().parents[1]
source = root / 'managed-client/core/build.rs'
with tempfile.TemporaryDirectory() as temporary:
    binary = Path(temporary) / 'build-check'
    subprocess.run(['rustc', '--edition=2021', str(source), '-o', str(binary)], check=True)
    env = {k: v for k, v in os.environ.items() if k not in ('RUSTDESK_CONTROL_URL', 'RUSTDESK_MANAGED_SOURCE_URL')}
    env['PROFILE'] = 'release'
    cases = [({}, False), ({'RUSTDESK_CONTROL_URL': 'https://control.example.com'}, False),
             ({'RUSTDESK_CONTROL_URL': 'http://control.example.com', 'RUSTDESK_MANAGED_SOURCE_URL': 'https://source.example.com'}, False),
             ({'RUSTDESK_CONTROL_URL': 'https://control.example.com', 'RUSTDESK_MANAGED_SOURCE_URL': 'https://source.example.com/modified'}, True)]
    for updates, success in cases:
        result = subprocess.run([str(binary)], env={**env, **updates}, capture_output=True)
        assert (result.returncode == 0) == success, 'release guard mismatch'
print('4 managed release environment checks passed')
