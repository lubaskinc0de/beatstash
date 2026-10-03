# Contributing to beatstash

Use the [local development guide](https://lubaskinc0de.github.io/beatstash/development/local/) to set up Go and the supporting services.

## Report a problem

Include your application revision, Navidrome version, deployment method, steps to reproduce, and expected result. Remove credentials and private data from configuration and logs before sharing them.

## Prepare a change

Work on a branch and keep the pull request focused on one problem. Explain what changes for the user and how you checked it.

For behavior changes, add an end-to-end business scenario at the bot boundary. Tests use real Postgres, Navidrome, files, and `ffmpeg`, with HTTP doubles for Telegram and streaming providers. Follow Arrange, Act, Assert and start with a failing scenario.

Run `just test`, `just lint`, and `just fmt` for Go changes. For documentation, run `npm ci` and `npm run check` from `docs`. State any checks you could not complete.

CI checks formatting, lint, `go vet`, module consistency, workflow syntax and security with actionlint and zizmor, spelling with typos, secret leaks with Gitleaks, the documentation build, and all end-to-end scenarios. Coverage measures application packages exercised by those scenarios; on `master` its total goes to the README badge through the `badges` branch. Run static checks locally with `just lint` and tests with `just test`.

New integrations need tests for their actual capabilities and failures, an updated source-support table, and a user setup guide. Full guidance is in [contributing documentation](https://lubaskinc0de.github.io/beatstash/development/contributing/).

Contributions are included under the project's [MIT license](LICENSE).

Maintainers can follow [CI and release setup](.github/README.md) to configure checks, coverage, and GHCR publication.
