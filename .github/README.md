# CI and releases

Pull requests and pushes to `master` run the reusable checks in `checks.yml`. The Go static job also runs actionlint, zizmor, typos, and release-policy tests. The other jobs run the complete Docker-backed end-to-end suite and build the documentation. Use `just lint` for all static checks and `just test` for tests after installing the tools in the [development guide](https://lubaskinc0de.github.io/beatstash/development/local/).

Dependabot checks Go modules, documentation npm packages, Dockerfile images, Compose images, and pinned GitHub Actions weekly. Minor and patch updates for Go, npm, and Actions are grouped; major updates remain separate. Version updates have a seven-day cooldown. Review changes to the tested Go and Navidrome versions together with their configuration and compatibility requirements.

## Repository setup

Enable GitHub Actions and configure a ruleset for `master` that requires the Go static checks, end-to-end tests, and documentation checks before merging. The workflow files run these checks, but cannot configure a ruleset for a repository that has not been created yet.

For documentation, choose **GitHub Actions** under **Settings > Pages**. The documentation workflow publishes the default branch.

During each Pages build, the workflow reads GitHub's latest stable release and sets `DOCS_RELEASE_TAG`. The site inserts that version into the installation and update instructions, including the copyable commands. Publishing the newest stable release dispatches the documentation workflow again; prereleases and older releases leave the selected version unchanged. Local previews without `DOCS_RELEASE_TAG` use `vX.Y.Z` until a release is selected. Before the first stable release, GitHub builds also use preview placeholders.

Enable `lubaskinc0de/beatstash` in Codecov. The coverage upload uses [OIDC](https://github.com/codecov/codecov-action#using-oidc), so no long-lived Codecov token is needed. Reports are uploaded after successful tests on `master`; pull requests and release tags still produce downloadable coverage artifacts. A Codecov outage does not block a release after tests pass.

After the first image publication, open the beatstash package under your GitHub profile, choose **Package settings > Change visibility**, and make it **Public**. GHCR initially creates packages as private, even for a public repository. This one-time setting is needed for anonymous installation; see [GitHub's package visibility instructions](https://docs.github.com/en/packages/learn-github-packages/configuring-a-packages-access-control-and-visibility). Confirm that the package is linked to this repository and grants its Actions workflows access.

## Publish a release

Merge the release changes into `master`, then tag the intended commit. For example, after replacing the example version with the version you intend to release:

```sh
git switch master
git pull --ff-only
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

Use `vX.Y.Z-rc.1` for a prerelease. Tags must follow SemVer without build metadata; prerelease versions use valid Docker tags. A tag can refer to an earlier commit in `master`, but a commit outside its history is rejected. Do not move published release tags.

The release workflow repeats all checks against the tagged commit, then builds and pushes one multi-platform image for Linux AMD64 and ARM64. The image version omits the leading `v`, and OCI labels record its version, source commit, license, and documentation URL. Release notes include the image digest and installation links.

Each GitHub Release includes `beatstash-vX.Y.Z-deploy.tar.gz` and its SHA-256 checksum. The archive contains `deploy/compose.yml`, `deploy/.env.example`, `deploy/config.example.toml`, and MIT. Its environment template already selects that release's image version. Credentials and music are never packaged.

Only the newest stable release updates the image's `latest` tag and GitHub's latest-release marker. Publishing an older stable version or a prerelease leaves the current latest release unchanged. Installation guides use exact versions.

If a workflow fails, inspect its logs before retrying it. **Actions > Release > Run workflow** accepts an existing tag and repeats the checks. Release notes and assets are regenerated on retry so the recorded image digest matches the rebuilt image.

The Docker build uses the runner's architecture to compile both target binaries. QEMU runs the ARM64 Alpine package-installation step. Root `docker-compose.yml` remains the development setup; released installations use `deploy/compose.yml` and pull their image from GHCR.

## Tool versions

CI uses the Go version from `go.mod`, Node 24, golangci-lint 2.13.2, actionlint 1.7.12, zizmor 1.30.1, and typos 1.50.3. Actions are pinned to commit hashes and updated by Dependabot.

actionlint 1.7.12 does not understand GitHub's `$/` self-repository workflow syntax. `.github/actionlint.yaml` ignores only those two parser errors. zizmor checks the same references with no suppressed security findings. Remove the parser exceptions when actionlint supports this syntax.

The release also includes `install.sh` and its SHA-256 checksum. The installer is pinned to that release and downloads the matching deployment archive. Its source is `deploy/install.sh`; CI checks it with Bash and ShellCheck and tests its configuration helpers without running the interactive setup. Published configuration templates use a placeholder administrator ID.
