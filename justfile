set dotenv-load := true
set shell := ["bash", "-uo", "pipefail", "-c"]

up:
    docker compose up postgres navidrome -d
    go run cmd/navidrome-tg/main.go

test:
    go test -count=1 -v ./e2e/ 2>&1 | grep -vE '^(\{|[0-9]{4}/|=== (RUN|PAUSE|CONT)|  [A-Z])'

fixtures:
    ./e2e/testdata/audio/generate.sh
