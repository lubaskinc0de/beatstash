#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

# History includes secrets deleted in later commits.
gitleaks git --config .gitleaks.toml --redact --no-banner --verbose --log-opts=--all .

# Copy the current files Git would include. Ignored local credentials and build
# outputs stay out; files already tracked by Git are included even if ignored.
scan_dir=$(mktemp -d)
trap 'rm -rf "$scan_dir"' EXIT
while IFS= read -r -d '' file; do
    [[ -f "$file" && ! -L "$file" ]] || continue
    mkdir -p "$scan_dir/$(dirname -- "$file")"
    cp -- "$file" "$scan_dir/$file"
done < <(git ls-files --cached --others --exclude-standard -z)

gitleaks dir --config .gitleaks.toml --redact --no-banner --verbose "$scan_dir"
