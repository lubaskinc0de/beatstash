---
title: Local development
description: Run the Go application with local services and execute its checks.
---

## Tools

Use the [Go](https://go.dev/doc/install) version declared in `go.mod`, currently **1.27.1**, plus [Docker Engine](https://docs.docker.com/engine/install/) with [Compose](https://docs.docker.com/compose/), [`ffmpeg`](https://ffmpeg.org/), [Bash](https://www.gnu.org/software/bash/), and [just](https://just.systems/man/en/). The lint tools are listed under [checks](#checks).

End-to-end tests use real [Postgres](https://www.postgresql.org/) and [Navidrome](https://www.navidrome.org/) containers, the real filesystem, and `ffmpeg`. Telegram and provider APIs are HTTP test doubles; no real bot token or Zvuk subscription is needed for the test suite.

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

For small files, leave `telegram.bot_api_url` empty and use the cloud API. For local API development, follow [Telegram setup](../installation/telegram.mdx) and configure host access plus identical local file paths; the production-style shared volume assumes the bot also runs in [Docker](https://docs.docker.com/). Use a separate development token rather than polling the production token simultaneously.

## Checks

```sh
just lint
just test
```

`just lint` runs Go lint and formatting checks, `go vet`, compilation, module checks, workflow linting, spelling, ShellCheck, installer syntax, and secret scanning. It leaves files unchanged; `just fmt` fixes Go formatting. `just docs` checks and builds this documentation.

`just lint` looks for tools on your `PATH`, in Go's `bin` folder, and in `~/.local/bin`, and lists any that are missing before it starts.

| Tool | Version | Install |
|---|---|---|
| [golangci-lint](https://golangci-lint.run/docs/welcome/install/) | v2 | Into `$(go env GOPATH)/bin` |
| [actionlint](https://github.com/rhysd/actionlint) | 1.7.12 | `go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` |
| [Gitleaks](https://github.com/gitleaks/gitleaks) | 8.30.1 | `go install github.com/zricethezav/gitleaks/v8@v8.30.1` |
| [ShellCheck](https://www.shellcheck.net/) | 0.11.0 | Your package manager, or `uv tool install shellcheck-py==0.11.0.1` |
| [zizmor](https://docs.zizmor.sh/) | 1.30.1 | `uv tool install zizmor==1.30.1`, or see its [installation guide](https://docs.zizmor.sh/installation/) |
| [typos](https://github.com/crate-ci/typos) | 1.50.3 | Binary from [release v1.50.3](https://github.com/crate-ci/typos/releases/tag/v1.50.3) into `~/.local/bin` |
| [Node](https://nodejs.org/en/download) | 24 | For the docs: `npm --prefix docs ci` |

Secret scanning covers all local Git history plus tracked and new files. Ignored local credentials and generated files are skipped, but a tracked file is scanned even if it matches `.gitignore`. Findings fail the check, with secret values redacted in the log. The only exception is the fixed test encryption key in the test harness.

`just test` runs release-policy tests, unit tests, the installer scenarios, and the end-to-end business scenarios. Docker must be running and your user must be able to use it. CI also collects coverage from the end-to-end tests.

The installer scenarios in `e2e/installer` run the real installer against Docker with real Navidrome and [Caddy](https://caddyserver.com/docs/) containers and stand-ins for the bot and Telegram. They use the `beatstash` Compose project, host ports 80 and 443, and a test Caddy on host ports 18080, 18443 and 12019, so they run one at a time and need those ports free. Stop any other Caddy containers first: the installer would find them. To try the installer by hand, build it with a version, such as `go build -ldflags "-X main.version=0.0.1" ./cmd/beatstash-setup`.

The small audio samples are committed to the repository, and tests generate additional audio as needed. You do not need to regenerate them before running tests. `ffmpeg` is still required.

For unfiltered test output or one test:

```sh
go test -count=1 ./e2e/
go test -count=1 ./e2e/ -run TestAttachedLibraryAccess
```

Test containers are isolated from the root development Compose stack. Container downloads may make the first test run slower.
