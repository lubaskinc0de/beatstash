import subprocess
import tarfile
import tempfile
import unittest
from pathlib import Path

import release


class ReleasePolicyTests(unittest.TestCase):
    def test_rejects_non_semver_and_unsafe_tags(self):
        for tag in ("1.2.3", "v01.2.3", "v1.2", "v1.2.3+build", "v1.2.3-rc.01", "v1.2.3\n", "v1.2.3;echo bad"):
            with self.subTest(tag=tag), self.assertRaises(ValueError):
                release.parse_tag(tag)

    def test_old_stable_and_prerelease_cannot_replace_latest(self):
        published = [{"tag_name": "v2.10.0"}, {"tag_name": "v3.0.0", "draft": True}, {"tag_name": "v4.0.0-rc.1", "prerelease": True}]
        self.assertFalse(release.is_latest("v2.9.0", published))
        self.assertFalse(release.is_latest("v5.0.0-rc.1", published))
        self.assertTrue(release.is_latest("v2.10.0", published))
        self.assertTrue(release.is_latest("v2.11.0", published))

    def test_accepts_ancestor_and_rejects_unmerged_tag(self):
        with tempfile.TemporaryDirectory() as directory:
            def git(*args):
                return subprocess.check_output(["git", "-C", directory, *args], stderr=subprocess.DEVNULL, text=True).strip()

            git("init", "-b", "master")
            git("config", "user.email", "test@example.com")
            git("config", "user.name", "Test")
            git("commit", "--allow-empty", "-m", "base")
            base = git("rev-parse", "HEAD")
            git("tag", "-a", "v1.0.0", "-m", "release")
            git("commit", "--allow-empty", "-m", "master update")
            git("update-ref", "refs/remotes/origin/master", "HEAD")
            git("checkout", "-b", "other", base)
            git("commit", "--allow-empty", "-m", "unmerged")
            git("tag", "v2.0.0")
            self.assertEqual(release.check_master("v1.0.0", cwd=directory), base)
            with self.assertRaises(subprocess.CalledProcessError):
                release.check_master("v2.0.0", cwd=directory)

    def test_archive_contains_only_templates_and_pins_version(self):
        root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as directory:
            archive = release.package("v1.2.3-rc.1", "Example/Beatstash", root, directory)
            with tarfile.open(archive) as tar:
                self.assertEqual(set(tar.getnames()), {"deploy/compose.yml", "deploy/.env.example", "deploy/config.example.toml", "LICENSE"})
                self.assertIn(b'BEATSTASH_VERSION="1.2.3-rc.1"', tar.extractfile("deploy/.env.example").read())
                self.assertIn(b"ghcr.io/example/beatstash:", tar.extractfile("deploy/compose.yml").read())
            self.assertTrue(Path(f"{archive}.sha256").exists())


if __name__ == "__main__":
    unittest.main()
