---
title: Contributing
description: Report a problem or prepare a code, integration, or documentation change.
---

## Report a problem

Include the application revision, Navidrome version, deployment method, the steps to reproduce, and the result you expected. Add relevant sanitized logs. For imports, name the source and whether the failure occurs during connection, planning, download, or sync.

Keep credentials and private music out of reports. A small reproducible example is more useful than a full production configuration.

## Change code

Set up [local development](./local.md) and work on a branch. Keep the change focused on one problem, and explain the resulting behavior in the pull request.

The project's tests are end-to-end business scenarios at the bot boundary. They run the real database, Navidrome, filesystem, and `ffmpeg`; HTTP doubles stand in for Telegram and external providers. For behavior changes, write an observable scenario using Arrange, Act, Assert and a failing test before the implementation. Avoid unit tests that only repeat a use case's internals.

Run `just test`, `just lint`, and `just fmt` for Go changes. Mention which checks ran and any check you could not complete. Do not mark a blocked check as passed.

CI runs formatting and lint checks, `go vet`, module verification, actionlint, zizmor, spelling checks with typos, documentation checks, and the complete end-to-end suite. Run static checks locally with `just lint` and tests with `just test`. Every release must pass the same checks before its Docker image is published. Coverage reports include the application packages exercised by the end-to-end scenarios.

## Add an integration

Check the provider interfaces and current Zvuk implementation before choosing a design. A source can support only some capabilities; distinguish collection metadata, audio download, and sync.

Cover connection failure, unavailable tracks, duplicates, quotas, and later collection changes in business-scenario tests. Update the [source table](../import/sources.md), add a setup guide, and describe the exact sync behavior and credentials it requires.

## License

beatstash is licensed under MIT. Contributions are included under the project's license.
