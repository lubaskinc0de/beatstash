#!/usr/bin/env bash
#
# Downloads the beatstash setup tool of this release, checks it against the
# checksum built into this script, and runs it with the given arguments:
#   bash install.sh            install, or continue an unfinished setup
#   bash install.sh upgrade    move an installation to this release
#   bash install.sh uninstall  remove an installation

set -euo pipefail

# Filled in by the release.
TAG='__BEATSTASH_INSTALLER_RELEASE_TAG__'
REPOSITORY='__BEATSTASH_INSTALLER_REPOSITORY__'
SHA256_AMD64='__BEATSTASH_SETUP_SHA256_AMD64__'
SHA256_ARM64='__BEATSTASH_SETUP_SHA256_ARM64__'

fail() { printf 'Error: %s\n' "$1" >&2; exit 1; }

[[ "$TAG" != __* ]] || fail 'Download install.sh from a beatstash release.'
[[ -t 0 ]] || fail 'Run bash install.sh from an interactive terminal.'
for tool in curl sha256sum; do command -v "$tool" >/dev/null || fail "Install $tool first."; done

case "$(uname -s)/$(uname -m)" in
  Linux/x86_64 | Linux/amd64) arch=amd64 checksum=$SHA256_AMD64 ;;
  Linux/aarch64 | Linux/arm64) arch=arm64 checksum=$SHA256_ARM64 ;;
  *) fail "beatstash runs on Linux AMD64 and ARM64, not $(uname -sm)." ;;
esac

scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
tool="$scratch/beatstash-setup"
curl --fail --silent --show-error --location \
  "https://github.com/$REPOSITORY/releases/download/$TAG/beatstash-setup-linux-$arch" -o "$tool"
printf '%s  %s\n' "$checksum" "$tool" | sha256sum --check --quiet - || fail 'The downloaded setup tool does not match its checksum.'
chmod +x "$tool"
"$tool" "$@"
