# Contributing to beatstash

Use the [local development guide](https://lubaskinc0de.github.io/beatstash/development/local/) to set up [Go](https://go.dev/doc/install) and the supporting services.

## Report a problem

Include your application revision, [Navidrome](https://www.navidrome.org/) version, deployment method, steps to reproduce, and expected result. Remove credentials and private data from configuration and logs before sharing them.

## Prepare a change

Work on a branch and keep the pull request focused on one problem. Explain what changes for the user and how you checked it.

For behavior changes, add an end-to-end business scenario at the bot boundary; for the setup tool, a scenario in `e2e/installer`. Tests use real [Postgres](https://www.postgresql.org/), Navidrome, files, and [`ffmpeg`](https://ffmpeg.org/), with HTTP doubles for Telegram and streaming providers. Follow Arrange, Act, Assert and start with a failing scenario.

Run `just test`, `just lint`, and `just fmt` for Go changes. For documentation, run `npm ci` and `npm run check` from `docs`. State any checks you could not complete.

CI checks formatting, lint, `go vet`, module consistency, workflow syntax and security with [actionlint](https://github.com/rhysd/actionlint) and [zizmor](https://docs.zizmor.sh/), spelling with [typos](https://github.com/crate-ci/typos), secret leaks with [Gitleaks](https://github.com/gitleaks/gitleaks), the documentation build, the setup tool's unit tests and scenarios, and all end-to-end scenarios. Coverage measures application and setup tool packages exercised by those scenarios; on `master` its total goes to the README badge through the `badges` branch. Run static checks locally with `just lint` and tests with `just test`.

New integrations need tests for their actual capabilities and failures, an updated source-support table, and a user setup guide. Full guidance is in [contributing documentation](https://lubaskinc0de.github.io/beatstash/development/contributing/).

Contributions are included under the project's [MIT license](LICENSE).

Maintainers can follow [CI and release setup](.github/README.md) to configure checks, coverage, and GHCR publication.
