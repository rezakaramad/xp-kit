<p align="center" width="100%">
	<img width="24%" src="./logo.png">
</p>
<p align="center" >
	<img src="https://img.shields.io/badge/go-00ADD8?style=flat&logo=go&logoColor=white" />
  <img src="https://img.shields.io/badge/crossplane-326CE5?style=flat&logo=crossplane&logoColor=white" />
	<img src="https://img.shields.io/badge/release--please-34A853?style=flat" />
	<img src="https://img.shields.io/badge/github%20actions-CI-2088FF?style=flat&logo=githubactions&logoColor=white" />
</p>

A home for Crossplane-related libraries, functions, code generation tools, and shared API types.

It is a small monorepo, and each top-level directory has a pretty specific job.

## Repository structure

| Path | What it is for |
| --- | --- |
| [functions/](./functions/) | Crossplane composition functions. |
| [modules/](./modules/) | Shared Go modules for the rest of the repository. |
| [cmd/](./cmd/) | Entry points for repo-owned CLI tools. |
| [types/](./types/) | Shared API types and schemas used by the rest of the repo. |
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

A few useful ones:

```sh
$ task --list
task: Available tasks for this project:
* check:function:               Run tidy, lint, and tests for one function module.
* check:functions:              Run tidy, lint, and tests for all function modules.
* check:xtenant-render:         Run checks for xtenant-render.
* check:xtenant-validate:       Run checks for xtenant-validate.

$ task check:xtenant-validate
$ task check:functions
```

## Release flow
There are two moving parts here:

- [release-please](https://github.com/googleapis/release-please) watches `main` and prepares version bumps and GitHub releases for the configured components.
- Package publication only happens when you push a component tag that matches the workflow pattern.

Use this sequence:

1. Push your work to a feature branch.
2. Open a PR into `main`.
3. Review and merge it.
4. Let [release-please](https://github.com/googleapis/release-please) open or update the release PR from `main`.
5. Merge the release PR.
6. Create and push only the component tags you want to publish from `main`.

For normal releases, do not create publish tags from feature branches.

**Tag patterns** used by the current workflows:

- Functions: `functions/<name>/v<semver>`
- CLI binaries: `cmd/<name>/v<semver>`
- Shared types: `types/xtenant/v<semver>`

Example:

```sh
git tag functions/xtenant-validate/v0.1.0
git tag functions/xtenant-render/v0.1.0
git push origin functions/xtenant-validate/v0.1.0 functions/xtenant-render/v0.1.0
```

What each tag does:

- `functions/xtenant-validate/v0.1.0` publishes `ghcr.io/<owner>/function-xtenant-validate:v0.1.0`
- `functions/xtenant-render/v0.1.0` publishes `ghcr.io/<owner>/function-xtenant-render:v0.1.0`
- `cmd/gen-xrd/v0.1.0` triggers the CLI binary release workflow and uploads archives plus checksums to the GitHub Release

For Go libraries under `modules/`, the semver tag is the release artifact for Go module consumers. There is no separate binary/package publishing workflow for those modules.

When you look at GitHub Packages, you will usually see both `function-...` and `...-runtime` images:

- `function-...` is the Crossplane function package you install from a `Function` resource. It is built from the `.xpkg` artifact.
- `...-runtime` is the backing container image used by that package.
- In normal usage, you want `function-...`, not `...-runtime`.

GitHub Releases and GHCR package versions are related, but they are not the same thing:

- GitHub Releases come from the release workflows.
- GHCR function package versions come directly from the function tag that triggered publication.
- If you want Releases and Packages to stay aligned, publish functions with the same semver that was just merged by [release-please](https://github.com/googleapis/release-please) for that component.

## Notes

- This repo uses multiple Go modules instead of one top-level module.
- Function packaging is validated on branch and pull request builds, then only published from function version tags.

Made with 🤓, 🐧 and 🍷.
