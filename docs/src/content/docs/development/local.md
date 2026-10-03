---
title: Local development
description: Run the Go application with local services and execute its checks.
---

## Tools

Use the Go version declared in `go.mod`, currently **1.27.1**, plus Docker Engine with Compose, `ffmpeg`, Bash, and [just](https://just.systems/man/en/). Install a compatible [golangci-lint](https://golangci-lint.run/docs/welcome/install/) v2 executable under `$(go env GOPATH)/bin` for the existing lint and formatting recipes.

End-to-end tests use real Postgres and Navidrome containers, the real filesystem, and `ffmpeg`. Telegram and provider APIs are HTTP test doubles; no real bot token or Zvuk subscription is needed for the test suite.

## Prepare configuration

From the repository root:

```sh
cp .env.example .env
cp config.example.toml config.toml
mkdir -p test_data/shared test_data/users
```

Fill startup credentials in `.env`. Generate `SECRET_KEY` with `openssl rand -base64 32`. For host-run development, keep the sample `DB_DSN` pointing to `localhost:5432`.

Edit your own administrator ID and these existing TOML settings:

```toml
[library]
music_dir = "/absolute/path/to/beatstash/test_data"
navidrome_music_dir = "/music"

[navidrome]
url = "http://localhost:4533"
public_url = ""
user = "admin"
```

Use an absolute host path for `music_dir`. The runtime does not expand `~` in that setting. The root Compose file exposes Postgres and Navidrome on the host for development; keep those ports confined to your development machine.

Start dependencies and create the first Navidrome administrator at `http://localhost:4533`:

```sh
docker compose up -d postgres navidrome
```

Use its login and password in `config.toml` and `.env`. Then:

```sh
just up
```

`just` loads `.env`; the Go application does not. `just up` starts Postgres and Navidrome, then runs `go run cmd/beatstash/main.go` on the host. It does not start the local Telegram API or a containerized bot.

For small files, leave `telegram.bot_api_url` empty and use the cloud API. For local API development, follow [Telegram setup](../installation/telegram.md) and configure host access plus identical local file paths; the production-style shared volume assumes the bot also runs in Docker. Use a separate development token rather than polling the production token simultaneously.

## Checks

```sh
just lint
just test
```

`just lint` runs Go lint, formatting checks, vet, compilation, module checks, actionlint, zizmor, typos, ShellCheck, installer syntax, and Gitleaks secret scanning. Use `just docs` to check and build the documentation. Install Gitleaks **8.30.1**, ShellCheck **0.11.0**, actionlint **1.7.12**, zizmor **1.30.1**, and typos **1.50.3** on your `PATH`, in addition to the Go lint tools. Install documentation dependencies with `npm --prefix docs ci` using Node 24.

Install actionlint with Go:

```sh
go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
```

Install Gitleaks with Go:

```sh
go install github.com/zricethezav/gitleaks/v8@v8.30.1
```

Secret scanning checks all locally available Git history and current tracked or new files. Ignored local credentials and generated files are excluded from the current-file scan. A tracked file is checked even if it matches `.gitignore`. Findings fail the check, and secret values are redacted in logs. The only project exception matches the fixed test encryption key in its current and former test-harness files.

For ShellCheck, use your package manager or `uv tool install shellcheck-py==0.11.0.1`.

If you use uv, install zizmor with `uv tool install zizmor==1.30.1`; other options are in its [installation guide](https://docs.zizmor.sh/installation/). Download the typos binary for your system from [release v1.50.3](https://github.com/crate-ci/typos/releases/tag/v1.50.3) and place it in `~/.local/bin`, or another directory on your `PATH`. `just lint` also searches Go's `bin` directory and `~/.local/bin`, and reports missing tools before starting the checks.

`just test` runs release-policy and installer helper tests, plus end-to-end business scenarios. Docker must be running and your user must be able to use it. CI also collects coverage from the end-to-end tests.

Use `just fmt` to change Go formatting. `just lint` leaves source files unchanged. The small audio samples are committed to the repository, and tests generate additional audio as needed. You do not need to regenerate them before running tests. `ffmpeg` is still required.

For unfiltered test output or one test:

```sh
go test -count=1 ./e2e/
go test -count=1 ./e2e/ -run TestAttachedLibraryAccess
```

Test containers are isolated from the root development Compose stack. Container downloads may make the first test run slower.
