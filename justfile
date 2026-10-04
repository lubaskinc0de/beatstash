set dotenv-load := true
set shell := ["bash", "-euo", "pipefail", "-c"]

gotestsum := "go run gotest.tools/gotestsum@v1.13.0 --format-hide-empty-pkg"

up:
    docker compose up postgres navidrome -d
    go run cmd/beatstash/main.go

test: test-unit test-e2e

test-unit:
    @printf '\nUnit tests\n'
    @python3 -m unittest discover -q -s .github/scripts -p 'test_*.py'
    @{{gotestsum}} --format pkgname -- -count=1 ./internal/...

# Run both end-to-end packages concurrently and wait for both results.
[parallel]
test-e2e: test-setup test-main

test-main:
    @printf '\nApplication scenarios\n'
    @{{gotestsum}} --format testname -- -count=1 -parallel=4 ./e2e/

lint:
    #!/usr/bin/env bash
    set -euo pipefail
    export PATH="$(go env GOPATH)/bin:$HOME/.local/bin:$PATH"
    missing_tools=()
    for tool in golangci-lint actionlint zizmor typos shellcheck gitleaks npm; do
        if ! command -v "$tool" > /dev/null 2>&1; then
            missing_tools+=("$tool")
        fi
    done
    if (( ${#missing_tools[@]} )); then
        printf 'Missing lint tools: %s\n' "${missing_tools[*]}" >&2
        printf 'Setup: https://lubaskinc0de.github.io/beatstash/development/local/\n' >&2
        exit 127
    fi
    bash .github/scripts/check-secrets.sh
    "$(go env GOPATH)/bin/golangci-lint" run ./...
    format_diff=$(mktemp)
    trap 'rm -f "$format_diff"' EXIT
    "$(go env GOPATH)/bin/golangci-lint" fmt --diff ./... > "$format_diff"
    if [[ -s "$format_diff" ]]; then
        cat "$format_diff"
        exit 1
    fi

    go vet ./...
    go build ./...
    go mod verify
    go mod tidy -diff
    bash -n deploy/install.sh
    bash -n .github/scripts/check-secrets.sh
    shellcheck deploy/install.sh .github/scripts/check-secrets.sh
    actionlint
    zizmor --offline --persona=auditor .github
    typos --hidden .

fmt:
    "$(go env GOPATH)/bin/golangci-lint" fmt ./...

# Setup tool scenarios alone, with isolated Docker stacks in parallel.
test-setup:
    @printf '\nInstaller scenarios\n'
    @{{gotestsum}} --format testname -- -count=1 ./e2e/installer/

# The version picks the bot image it installs: `just setup-tool 0.0.1 arm64`.
# Builds the setup tool for a server without a release.
setup-tool version arch="amd64":
    CGO_ENABLED=0 GOOS=linux GOARCH={{arch}} go build -trimpath \
        -ldflags "-s -w -X main.version={{version}}" \
        -o bin/beatstash-setup-linux-{{arch}} ./cmd/beatstash-setup

docs:
    npm --prefix docs run check
    cd docs; npm run build
