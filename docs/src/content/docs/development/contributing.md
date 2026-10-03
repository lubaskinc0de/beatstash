---
title: Contributing
description: Report a problem or prepare a code, integration, or documentation change.
---

## Report a problem

Include the application revision, [Navidrome](https://www.navidrome.org/) version, deployment method, the steps to reproduce, and the result you expected. Add relevant sanitized logs. For imports, name the source and whether the failure occurs during connection, planning, download, or sync.

Keep credentials and private music out of reports. A small reproducible example is more useful than a full production configuration.

## Change code

Set up [local development](./local.md) and work on a branch. Keep the change focused on one problem, and explain the resulting behavior in the pull request.

The project's tests are end-to-end business scenarios at the bot boundary. They run the real database, Navidrome, filesystem, and [`ffmpeg`](https://ffmpeg.org/); HTTP doubles stand in for Telegram and external providers. For behavior changes, write an observable scenario using Arrange, Act, Assert and a failing test before the implementation. Avoid unit tests that only repeat a use case's internals.

The setup tool has its own scenarios in `e2e/installer`: they run the installer with typed answers against real [Docker](https://docs.docker.com/), Navidrome, and [Caddy](https://caddyserver.com/docs/), with stand-ins for the bot and Telegram. Unit tests are kept for its pure text edits only, the Caddyfile block and `config.toml` merging, where a scenario per edge case would be slow.

Run `just test`, `just lint`, and `just fmt` for [Go](https://go.dev/doc/install) changes. Mention which checks ran and any check you could not complete. Do not mark a blocked check as passed.

CI runs formatting and lint checks, `go vet`, module verification, [actionlint](https://github.com/rhysd/actionlint), [zizmor](https://docs.zizmor.sh/), spelling checks with [typos](https://github.com/crate-ci/typos), documentation checks, the setup tool tests, and the complete end-to-end suite. Run static checks locally with `just lint` and tests with `just test`. Every release must pass the same checks before its Docker image is published. Coverage reports include the application and setup tool packages exercised by the scenarios.

## Add an integration

Check the provider interfaces and current Zvuk implementation before choosing a design. A source can support only some capabilities; distinguish collection metadata, audio download, and sync.

Cover connection failure, unavailable tracks, duplicates, quotas, and later collection changes in business-scenario tests. Update the [source table](../import/sources.md), add a setup guide, and describe the exact sync behavior and credentials it requires.

## License

beatstash is licensed under MIT. Contributions are included under the project's license.
