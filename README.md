<p align="center" width="100%">
	<img width="24%" src="./logo.png">
</p>
<p align="center" >
	<img src="https://img.shields.io/badge/go-00ADD8?style=flat&logo=go&logoColor=white" />
  <img src="https://img.shields.io/badge/crossplane-326CE5?style=flat&logo=crossplane&logoColor=white" />
	<img src="https://img.shields.io/badge/release--please-34A853?style=flat" />
	<img src="https://img.shields.io/badge/github%20actions-CI-2088FF?style=flat&logo=githubactions&logoColor=white" />
</p>

A monorepo for Crossplane-related libraries, functions, code generation tools, and shared API types.

Each top-level directory has a pretty specific job.

## Repository structure

| Path | What it is for |
| --- | --- |
| [functions/](./functions/) | Crossplane composition functions. |
| [modules/](./modules/) | Shared Go modules for the rest of the repository. |
| [cmd/](./cmd/) | Entry points for repo-owned CLI tools. |
| [types/](./types/) | Shared API types and schemas used by the rest of the repo. |
| [hack/](./hack/) | Internal development scripts such as local fix and verify helpers. |
| [.github/](.github/) | Automation for CI, releases, and package publishing. |

## Typical workflow

| If you want to... | Start here |
| --- | --- |
| Add a new type | [types/README.md](./types/README.md) |
| Add a new function | [functions/README.md](./functions/README.md) |
| Use `runner` inside a function | [modules/runner/README.md](./modules/runner/README.md) |
| Work on generation tooling | [cmd/gen-xrd/README.md](./cmd/gen-xrd/README.md) |

## Local checks

- VS Code is set up to use [golangci-lint](https://github.com/golangci/golangci-lint) on save.
- We use a [Taskfile](https://taskfile.dev) to keep the common checks in one place.
- Run `task --list` to see what is available, then pick the check you need for one function or for all of them.
- The underlying scripts live in [hack/](./hack/), following the Kubernetes/CNCF convention for internal dev tooling.
- [Taskfile.yml](./Taskfile.yml) is the real source of truth for local commands.
- [.vscode/tasks.json](./.vscode/tasks.json) is only VS Code convenience. It makes the same commands clickable from `Tasks: Run Task`, but it duplicates what the Taskfile already defines.


A few useful ones:

```sh
$ task --list
task: Available tasks for this project:
* check:function:               Run tidy, lint, and tests for one function module.
* check:functions:              Run tidy, lint, and tests for all function modules.
* check:xtenant-render:         Run checks for xtenant-render.
* check:xtenant-validate:       Run checks for xtenant-validate.
* fix:function:                 Auto-fix tidy, lint, and formatting issues for one function module.
* fix:functions:                Auto-fix tidy, lint, and formatting issues for all function modules.
* fix:xtenant-render:           Auto-fix xtenant-render.
* fix:xtenant-validate:         Auto-fix xtenant-validate.

$ task fix:xtenant-validate
$ task check:xtenant-validate
$ task check:functions
```

## Release flow
There are two moving parts here:

- [release-please](https://github.com/googleapis/release-please) watches `main`, updates the component changelogs, and creates the GitHub Release entries for the configured components.
- Only some components have a tag-triggered publish workflow. For the rest, the version tag itself is the release artifact for Go module consumers.

Use this sequence:

1. Push your work to a feature branch.
2. Open a PR into `main`.
3. Review and merge it.
4. Let [release-please](https://github.com/googleapis/release-please) open or update the release PR from `main`.
5. Merge the release PR.
6. Create and push only the component tags you want to publish from `main`.

For normal releases, do not create publish tags from feature branches.

The version source of truth is:

- `release-please-config.json` defines which components participate in releases.
- `.release-please-manifest.json` records the current released version for each component.
- If you intentionally reset release history or want to restart from a new baseline, update `.release-please-manifest.json` before creating new tags.

**Release tags** used in this repo:

- Functions: `functions/<name>/v<semver>`
- CLI binaries: `cmd/<name>/v<semver>`
- Go libraries: `modules/<name>/v<semver>` and `types/<name>/v<semver>`

Example:

```sh
git tag functions/xtenant-validate/v0.0.1
git tag functions/xtenant-render/v0.0.1
git push origin functions/xtenant-validate/v0.0.1 functions/xtenant-render/v0.0.1
```

What each tag does:

- `functions/xtenant-validate/v0.0.1` publishes `ghcr.io/<owner>/function-xtenant-validate:v0.0.1`
- `functions/xtenant-render/v0.0.1` publishes `ghcr.io/<owner>/function-xtenant-render:v0.0.1`
- `cmd/gen-xrd/v0.0.1` triggers the CLI binary release workflow and uploads archives plus checksums to the GitHub Release
- `modules/runner/v0.0.1` creates the version tag consumed by `go get`; there is no separate binary/package publishing workflow
- `types/xtenant/v0.0.1` creates the version tag consumed by `go get`; there is no separate binary/package publishing workflow

What gets published for each component type:

- Functions: GitHub Release entry from `release-please`, plus GHCR function package and runtime image from the function tag.
- CLI tools under `cmd/`: GitHub Release entry from `release-please`, plus release assets (archives and checksums) from the CLI tag.
- Go libraries under `modules/` and `types/`: GitHub Release entry from `release-please`, plus the semver git tag consumed by the Go module system. No separate binary or OCI package is published.

When you look at GitHub Packages, you will usually see both `function-...` and `...-runtime` images:

- `function-...` is the Crossplane function package you install from a `Function` resource. It is built from the `.xpkg` artifact.
- `...-runtime` is the backing container image used by that package.
- In normal usage, you want `function-...`, not `...-runtime`.

GitHub Releases and GHCR package versions are related, but they are not the same thing:

- GitHub Releases come from the release workflows.
- GHCR function package versions come directly from the function tag that triggered publication.
- Go module consumers resolve `modules/*` and `types/*` directly from the matching semver tag.
- If you want Releases and Packages to stay aligned, publish functions with the same semver that was just merged by [release-please](https://github.com/googleapis/release-please) for that component.
- Function package files are not uploaded to the GitHub Release page; the installable artifacts live in GHCR.

### Verification `gh` commands

- List current GitHub Releases:

```sh
gh release list --limit 20
```

- Inspect the assets attached to a CLI release:

```sh
gh release view cmd/gen-xrd/v0.0.1 --json assets \
	| jq -r '.assets[] | [.name, .size] | @tsv'
```

- List recent GitHub Actions runs across workflows:

```sh
gh run list --limit 20 --json databaseId,workflowName,status,conclusion,headBranch,event \
	| jq -r '.[] | [.databaseId, .workflowName, .status, .conclusion, .headBranch, .event] | @tsv'
```

- List recent runs for the CLI binary workflow only:

```sh
gh run list --workflow "Release CLI Binaries" --limit 10
```

- List recent runs for the function package workflow only:

```sh
gh run list --workflow "Build and publish Crossplane function packages" --limit 10
```

- Inspect one workflow run in detail:

```sh
gh run view <run-id> --json databaseId,status,conclusion,headBranch,url,jobs
```

- Show failed logs for one workflow run:

```sh
gh run view <run-id> --log-failed
```

- List GHCR packages published from this repository:

```sh
gh api '/users/<owner>/packages?package_type=container&per_page=100' \
	--jq '.[] | select(.repository.full_name == "<owner>/<repo>") | .name'
```

- Example for this repository:

```sh
gh api '/users/rezakaramad/packages?package_type=container&per_page=100' \
	--jq '.[] | select(.repository.full_name == "rezakaramad/crossplane-toolkit") | .name'
```

- Inspect published assets and packages together after a release:

```sh
gh release view cmd/gen-xrd/v0.0.1 --json assets \
	| jq -r '.assets[] | [.name, .size] | @tsv'

gh api '/users/rezakaramad/packages?package_type=container&per_page=100' \
	--jq '.[] | select(.repository.full_name == "rezakaramad/crossplane-toolkit") | .name'
```

Why this repo uses a raw Go build workflow instead of GoReleaser for `cmd/*`:

- The original monorepo-oriented GoReleaser setup depended on the `monorepo` feature, which is only available in GoReleaser Pro.
- This repo already uses [release-please](https://github.com/googleapis/release-please) for versioning, changelogs, and GitHub Release entries, so GoReleaser's release-note generation was redundant.
- For `cmd/*`, the actual need is simple: build cross-platform binaries, archive them, generate checksums, and upload them to the existing GitHub Release.
- A plain `go build` based workflow does that directly, keeps the configuration small, and avoids a paid tool dependency.

## Notes

- This repo uses multiple Go modules instead of one top-level module.
- Function packaging is validated on branch and pull request builds, then only published from function version tags.

Made with 🤓, 🐧 and 🍷.
