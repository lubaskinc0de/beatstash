"""Validate release tags, select latest, and package installation files."""

import argparse
import hashlib
import io
import json
import re
import subprocess
import sys
import tarfile
from pathlib import Path

TAG = re.compile(r"v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?\Z")


def parse_tag(tag):
    match = TAG.fullmatch(tag)
    if not match:
        raise ValueError(f"Invalid release tag: {tag}. Use vMAJOR.MINOR.PATCH[-prerelease].")
    prerelease = match[4]
    if prerelease and any(part.isdigit() and len(part) > 1 and part[0] == "0" for part in prerelease.split(".")):
        raise ValueError("Numeric prerelease identifiers cannot have leading zeros.")
    if len(tag) - 1 > 128:
        raise ValueError("Version exceeds the Docker tag length limit.")
    return tuple(int(match[i]) for i in (1, 2, 3)), bool(prerelease)


def is_latest(tag, releases):
    version, prerelease = parse_tag(tag)
    if prerelease:
        return False
    for release in releases:
        if release.get("draft") or release.get("prerelease"):
            continue
        try:
            other, preview = parse_tag(release["tag_name"])
        except (KeyError, ValueError):
            continue
        if not preview and other > version:
            return False
    return True


def check_master(tag, cwd=None):
    parse_tag(tag)
    commit = subprocess.check_output(["git", "rev-parse", f"refs/tags/{tag}^{{commit}}"], text=True, cwd=cwd).strip()
    subprocess.run(["git", "merge-base", "--is-ancestor", commit, "refs/remotes/origin/master"], check=True, cwd=cwd)
    return commit


def package(tag, repository, root, output):
    parse_tag(tag)
    root, output = Path(root), Path(output)
    output.mkdir(parents=True, exist_ok=True)
    archive = output / f"beatstash-{tag}-deploy.tar.gz"
    files = {
        "deploy/compose.yml": root / "deploy/compose.yml",
        "deploy/compose.telegram-proxy.yml": root / "deploy/compose.telegram-proxy.yml",
        "deploy/.env.example": root / "deploy/.env.example",
        "deploy/config.example.toml": root / "config.example.toml",
        "LICENSE": root / "LICENSE",
    }
    with tarfile.open(archive, "w:gz") as tar:
        for name, source in files.items():
            content = source.read_text()
            if name.endswith(".env.example"):
                content = content.replace('BEATSTASH_VERSION=""', f'BEATSTASH_VERSION="{tag[1:]}"')
            if name.endswith("config.example.toml"):
                content = re.sub(r'^admins = .*', 'admins = ["telegram:123456789"]', content, flags=re.MULTILINE)
            if name.endswith("compose.yml"):
                content = content.replace("ghcr.io/lubaskinc0de/beatstash:", f"ghcr.io/{repository.lower()}:")
            data = content.encode()
            info = tarfile.TarInfo(name)
            info.size = len(data)
            info.mode = 0o644
            tar.addfile(info, io.BytesIO(data))
    checksum = hashlib.sha256(archive.read_bytes()).hexdigest()
    (output / f"{archive.name}.sha256").write_text(f"{checksum}  {archive.name}\n")
    installer = output / "install.sh"
    script = ((root / "deploy/install.sh").read_text()
              .replace("__BEATSTASH_INSTALLER_RELEASE_TAG__", tag)
              .replace("__BEATSTASH_INSTALLER_REPOSITORY__", repository))
    # The setup tool binaries are built into the output directory first.
    for arch in ("amd64", "arm64"):
        tool = output / f"beatstash-setup-linux-{arch}"
        if not tool.is_file():
            raise ValueError(f"Build {tool.name} before packaging.")
        digest = hashlib.sha256(tool.read_bytes()).hexdigest()
        (output / f"{tool.name}.sha256").write_text(f"{digest}  {tool.name}\n")
        script = script.replace(f"__BEATSTASH_SETUP_SHA256_{arch.upper()}__", digest)
    installer.write_text(script)
    installer.chmod(0o755)
    digest = hashlib.sha256(installer.read_bytes()).hexdigest()
    (output / "install.sh.sha256").write_text(f"{digest}  install.sh\n")
    return archive


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=["validate", "latest", "package"])
    parser.add_argument("tag")
    parser.add_argument("--repository", default="lubaskinc0de/beatstash")
    parser.add_argument("--root", default=".")
    parser.add_argument("--output", default="release-assets")
    args = parser.parse_args()
    try:
        _, prerelease = parse_tag(args.tag)
        if args.command == "validate":
            commit = check_master(args.tag)
            print(f"version={args.tag[1:]}\nprerelease={str(prerelease).lower()}\ncommit={commit}\nimage=ghcr.io/{args.repository.lower()}")
        elif args.command == "latest":
            print(str(is_latest(args.tag, json.load(sys.stdin))).lower())
        else:
            print(package(args.tag, args.repository, args.root, args.output))
    except (ValueError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"Release rejected: {error}\n")


if __name__ == "__main__":
    main()
