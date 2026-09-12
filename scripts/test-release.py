#!/usr/bin/env python3
"""Offline regression checks for release publication and source packaging."""
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]


def load(name):
    spec = importlib.util.spec_from_file_location(name, ROOT / 'scripts' / f'{name}.py')
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


publish = load('publish-release')
source = load('package-source')


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.previous = Path.cwd()
        os.chdir(self.tmp.name)
        Path('dist/release').mkdir(parents=True)
        for name in publish.ASSETS:
            Path('dist/release', name).write_bytes(b'test-binary')
        self.env = patch.dict(os.environ, RELEASE_TAG='v1.0.0', RELEASE_COMMIT='abc123', GH_REPO='test/project')
        self.env.start()

    def tearDown(self):
        self.env.stop()
        os.chdir(self.previous)
        self.tmp.cleanup()

    def test_publishes_only_after_both_assets_verified(self):
        with patch.object(publish, 'gh', side_effect=['[[]]', '', '', json.dumps({'assets': [{'name': n} for n in publish.ASSETS]}), '']) as gh:
            publish.main()
        calls = [c.args for c in gh.call_args_list]
        self.assertIn('--draft', calls[1])
        self.assertEqual(calls[-1], ('release', 'edit', 'v1.0.0', '--draft=false'))
        self.assertEqual(calls[2][3:-1], tuple('dist/release/' + n for n in publish.ASSETS))

    def test_refuses_extra_or_missing_files_before_network(self):
        Path('dist/release/extra.zip').touch()
        with patch.object(publish, 'gh') as gh, self.assertRaises(SystemExit):
            publish.main()
        gh.assert_not_called()
        Path('dist/release/extra.zip').unlink()
        Path('dist/release', publish.ASSETS[0]).unlink()
        with patch.object(publish, 'gh') as gh, self.assertRaises(SystemExit):
            publish.main()
        gh.assert_not_called()

    def test_never_overwrites_published_release(self):
        with patch.object(publish, 'gh', return_value='[[{"tag_name":"v1.0.0","draft":false}]]') as gh, self.assertRaises(SystemExit):
            publish.main()
        self.assertEqual(gh.call_count, 1)

    def test_asset_mismatch_keeps_draft(self):
        with patch.object(publish, 'gh', side_effect=['[[]]', '', '', '{"assets":[]}']) as gh, self.assertRaises(SystemExit):
            publish.main()
        self.assertFalse(any('--draft=false' in c.args for c in gh.call_args_list))

    def test_upload_failure_keeps_draft(self):
        with patch.object(publish, 'gh', side_effect=['[[]]', '', subprocess.CalledProcessError(1, 'gh')]) as gh, self.assertRaises(subprocess.CalledProcessError):
            publish.main()
        self.assertFalse(any('--draft=false' in c.args for c in gh.call_args_list))

    def test_existing_draft_can_resume(self):
        existing = [[{'tag_name': 'v1.0.0', 'draft': True, 'assets': [{'name': publish.ASSETS[0]}]}]]
        with patch.object(publish, 'gh', side_effect=[json.dumps(existing), '', '', json.dumps({'assets': [{'name': n} for n in publish.ASSETS]}), '']) as gh:
            publish.main()
        self.assertEqual(gh.call_args_list[1].args[:2], ('release', 'edit'))

    def test_unknown_draft_assets_are_preserved(self):
        existing = [[{'tag_name': 'v1.0.0', 'draft': True, 'assets': [{'name': 'other.zip'}]}]]
        with patch.object(publish, 'gh', return_value=json.dumps(existing)) as gh, self.assertRaises(SystemExit):
            publish.main()
        self.assertEqual(gh.call_count, 1)


class SourceTests(unittest.TestCase):
    def test_packages_modified_and_new_source_excludes_ignored_data(self):
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            subprocess.run(['git', 'init', '-q', tmp], check=True)
            (repo / '.gitignore').write_text('.env\nbuild/\n')
            (repo / 'tracked.rs').write_text('original')
            subprocess.run(['git', '-C', tmp, 'add', '.'], check=True)
            (repo / 'tracked.rs').write_text('modified')
            (repo / 'new.rs').write_text('new module')
            (repo / '.env').write_text('secret')
            (repo / 'build').mkdir()
            (repo / 'build' / 'binary').write_text('output')
            (repo / 'nested' / 'target').mkdir(parents=True)
            (repo / 'nested' / 'target' / 'cache').write_text('unignored build cache')
            (repo / 'link').symlink_to('/etc/passwd')
            data = io.BytesIO()
            with tarfile.open(fileobj=data, mode='w') as archive:
                source.add_tree(archive, repo, 'source')
            data.seek(0)
            with tarfile.open(fileobj=data) as archive:
                self.assertEqual(archive.extractfile('source/tracked.rs').read(), b'modified')
                self.assertEqual(archive.extractfile('source/new.rs').read(), b'new module')
                self.assertTrue(archive.getmember('source/link').issym())
                self.assertNotIn('source/.env', archive.getnames())
                self.assertNotIn('source/build/binary', archive.getnames())
                self.assertNotIn('source/nested/target/cache', archive.getnames())
                self.assertFalse(any('/.git/' in n for n in archive.getnames()))


if __name__ == '__main__':
    unittest.main()
