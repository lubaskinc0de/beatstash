set dotenv-load := true
set shell := ["bash", "-euo", "pipefail", "-c"]

up:
    docker compose up postgres navidrome -d
    go run cmd/beatstash/main.go

test:
    python3 -m unittest discover -s .github/scripts -p 'test_*.py'
    go test -count=1 -v ./e2e/ 2>&1 | grep -vE '^(\{|[0-9]{4}/|=== (RUN|PAUSE|CONT)|  [A-Z])'

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

docs:
    npm --prefix docs run check
    cd docs; npm run build