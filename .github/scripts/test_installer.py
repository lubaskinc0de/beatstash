"""Exercise installer helpers without running the interactive setup."""

import hashlib
import os
import subprocess
import tarfile
import tempfile
import unittest
from pathlib import Path

import release
import tomllib

ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / "deploy/install.sh"
HELPERS = SCRIPT.read_text().rsplit('\nmain "$@"', 1)[0]


class InstallerTests(unittest.TestCase):
    def run_helper(self, code, *args, directory=None):
        return subprocess.run(["bash", "-c", HELPERS + "\n" + code, "test", *args],
                              cwd=directory, text=True, capture_output=True)

    def test_secrets_round_trip_without_evaluation(self):
        with tempfile.TemporaryDirectory() as directory:
            value = "don't expand $HOME, `id`, $(touch nope), \\path # comment"
            result = self.run_helper('ENV_FILE="$1/settings"; save_value PASSWORD "$2" >/dev/null; get_value PASSWORD', directory, value)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, value)
            self.assertFalse((Path(directory) / "nope").exists())
            self.assertEqual((Path(directory) / "settings").stat().st_mode & 0o777, 0o600)

    def test_enter_keeps_a_saved_secret(self):
        with tempfile.TemporaryDirectory() as directory:
            result = self.run_helper('ENV_FILE="$1/settings"; save_value PASSWORD "$2" >/dev/null; '
                                     'prompt_value PASSWORD Password "" secret <<< "" >/dev/null; printf "%s" "$PASSWORD"',
                                     directory, "a'b$literal")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, "a'b$literal")

    def test_special_passwords_round_trip_through_compose_and_resume(self):
        values = (
            "ends\\", "ends\\\\", "slash\\'quote", "'", '"', "\\",
            '$HOME ${HOME} $$ $(touch nope)', 'quote"and\\slash',
            "literal\\n\\t\\r", "  spaces # comment  ", "slash\\$HOME",
        )
        with tempfile.TemporaryDirectory() as directory:
            env_file = Path(directory) / ".env"
            compose_file = Path(directory) / "compose.yml"
            compose_file.write_text('services:\n  check:\n    image: alpine\n'
                                    '    environment:\n      PASSWORD: ${PASSWORD}\n')
            environment = os.environ.copy()
            environment.pop("PASSWORD", None)
            for value in values:
                with self.subTest(value=value):
                    result = self.run_helper('ENV_FILE="$1"; save_value PASSWORD "$2" >/dev/null; '
                                             'prompt_value PASSWORD Password "" secret <<< "" >/dev/null; '
                                             'printf "%s" "$PASSWORD"', str(env_file), value, directory=directory)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout, value)
                    result = subprocess.run(['docker', 'compose', '--env-file', str(env_file),
                                             '-f', str(compose_file), 'config', '--environment'],
                                            env=environment, text=True, capture_output=True)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    resolved = next(line.removeprefix('PASSWORD=') for line in result.stdout.splitlines()
                                    if line.startswith('PASSWORD='))
                    self.assertEqual(resolved, value)
                    self.assertFalse((Path(directory) / "nope").exists())

    def test_reads_values_saved_by_the_previous_installer(self):
        for encoded, expected in (("'a\\'b$literal'", "a'b$literal"), ('"1.2.3"', '1.2.3'), ('""', '')):
            with self.subTest(encoded=encoded):
                result = self.run_helper('decode_env "$1"', encoded)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout, expected)

    def test_toml_update_preserves_other_settings(self):
        with tempfile.TemporaryDirectory() as directory:
            config = Path(directory) / "config.toml"
            config.write_text('[navidrome]\nuser = "admin"\naccess_ttl = "2m"\n[quota]\ndefault = "8GB"\n')
            value = 'user "quotes" \\ backslash'
            result = self.run_helper('CONFIG_FILE="$1"; set_config navidrome user "$(toml_string "$2")"', str(config), value)
            self.assertEqual(result.returncode, 0, result.stderr)
            data = tomllib.loads(config.read_text())
            self.assertEqual(data["navidrome"], {"user": value, "access_ttl": "2m"})
            self.assertEqual(data["quota"]["default"], "8GB")

    def test_existing_secret_is_not_regenerated(self):
        with tempfile.TemporaryDirectory() as directory:
            result = self.run_helper('ENV_FILE="$1/settings"; save_value SECRET_KEY "$2" >/dev/null; '
                                     'docker() { return 0; }; ensure_secret SECRET_KEY -base64; get_value SECRET_KEY',
                                     directory, "original encryption key")
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, "original encryption key")
            missing = self.run_helper('ENV_FILE="$1/missing"; docker() { return 0; }; ensure_secret SECRET_KEY -base64', directory)
            self.assertNotEqual(missing.returncode, 0)
            self.assertIn("Restore its original .env", missing.stderr)

    def test_archive_rejects_traversal_and_links(self):
        with tempfile.TemporaryDirectory() as directory:
            archive = release.package("v1.2.3", "lubaskinc0de/beatstash", ROOT, directory)
            result = self.run_helper('verify_archive "$1"', str(archive))
            self.assertEqual(result.returncode, 0, result.stderr)
            for linked in (False, True):
                unsafe = Path(directory) / "unsafe.tar.gz"
                with tarfile.open(unsafe, "w:gz") as tar:
                    for name in ("LICENSE", "deploy/.env.example", "deploy/compose.yml", "deploy/config.example.toml"):
                        info = tarfile.TarInfo(name)
                        if linked and name == "LICENSE":
                            info.type = tarfile.SYMTYPE
                            info.linkname = "/etc/passwd"
                        tar.addfile(info)
                    if not linked:
                        tar.addfile(tarfile.TarInfo("../escape"))
                result = self.run_helper('verify_archive "$1"', str(unsafe))
                self.assertNotEqual(result.returncode, 0)

    def test_rejects_a_stack_owned_by_another_directory(self):
        result = self.run_helper('DEPLOY_DIR=/wanted; docker() { '
                                 'if [[ "$1" == ps ]]; then printf "container\\n"; '
                                 'else printf "/another-install\\n"; fi; }; check_project')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Another beatstash stack", result.stderr)

    def test_release_installer_is_pinned_and_checksummed(self):
        with tempfile.TemporaryDirectory() as directory:
            archive = release.package("v2.3.4", "Example/Beatstash", ROOT, directory)
            installer = Path(directory) / "install.sh"
            self.assertIn("RELEASE_TAG='v2.3.4'", installer.read_text())
            self.assertIn("REPOSITORY='Example/Beatstash'", installer.read_text())
            self.assertTrue(os.access(installer, os.X_OK))
            expected = hashlib.sha256(installer.read_bytes()).hexdigest()
            self.assertEqual((Path(directory) / "install.sh.sha256").read_text(), f"{expected}  install.sh\n")
            with tarfile.open(archive) as tar:
                config = tomllib.loads(tar.extractfile("deploy/config.example.toml").read().decode())
                self.assertEqual(config["admins"], ["telegram:123456789"])


if __name__ == "__main__":
    unittest.main()
