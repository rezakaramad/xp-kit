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

## Functions: build and publish

Each function tag produces two GHCR artifacts:

- **`ghcr.io/<owner>/<name>-runtime:<version>`** — distroless image with the Go binary (embedded, never installed directly)
- **`ghcr.io/<owner>/function-<name>:<version>`** — the `.xpkg` Crossplane package that bundles the runtime image + `package/crossplane.yaml`; this is what you install on the cluster via a `Function` resource

Think of `.xpkg` like a Docker image for Crossplane: it packages your binary with the metadata Crossplane needs to run it as a composition function.

**Required files per function:**

```
functions/<name>/
├── Dockerfile            # two-stage: golang → distroless
├── package/
│   └── crossplane.yaml   # name + capabilities; name must match functionRef.name in Compositions
└── go.mod
```

**Adding a new function:** create the directory above, add it to `go.work` and to `release-please-config.json` + `.release-please-manifest.json` at `0.0.0`. The build workflow auto-discovers all `functions/` subdirectories.

## Release flow

**What's automatic vs manual:**

| Step | Who | Result |
| --- | --- | --- |
| Open release PR | release-please (on merge to `main`) | Bumped changelogs + `.release-please-manifest.json` |
| Merge release PR | **You** | GitHub Releases created |
| Push library tags | **You** | `types/*` and `modules/*` semver tags indexed by Go proxy |
| Push function tags | **You** | Build workflow → runtime image + Crossplane package in GHCR |

**Tag ordering** — the Go proxy caches versions immutably, so always tag in this order:

```
types/* → modules/* → functions/*
```

Never delete and recreate a tag — bump the patch version instead. CI is unaffected because `go.work` is committed and resolves all modules locally.

**Steps:**

1. Merge your PR to `main`; update function `go.mod` to the upcoming library version first if you changed a library API (CI compiles fine via `go.work`)
2. Merge the release PR opened by release-please
3. Tag and push libraries, verify proxy:
   ```sh
   git pull && git tag types/xtenant/v0.2.0 && git push origin types/xtenant/v0.2.0
   curl -s "https://proxy.golang.org/github.com/rezakaramad/crossplane-toolkit/types/xtenant/@v/v0.2.0.info"
   # must return {"Version":"v0.2.0",...} before continuing
   ```
4. Tag and push functions:
   ```sh
   git tag functions/xtenant-render/v0.1.0 functions/xtenant-validate/v0.1.0
   git push origin functions/xtenant-render/v0.1.0 functions/xtenant-validate/v0.1.0
   ```

> **release-please tags don't trigger the build workflow** — GitHub blocks `GITHUB_TOKEN` from triggering workflows. If tags already exist and `git push` says "Everything up-to-date", dispatch manually:
> ```sh
> gh workflow run "Build and publish Crossplane function packages" --ref functions/<name>/v<version>
> ```

> Commit scopes must match package paths exactly (`feat!(types/xtenant):` not `feat!(xtenant):`) or release-please misses the component.

**Letting Copilot do this:** say *"The release PR is merged — do the release"* and it will handle tag ordering and proxy verification automatically.

**Useful commands:**
```sh
gh run list --workflow "Build and publish Crossplane function packages" --limit 10
gh run view <run-id> --log-failed
gh release list --limit 20
gh api '/users/rezakaramad/packages?package_type=container&per_page=100' \
  --jq '.[] | select(.repository.full_name == "rezakaramad/crossplane-toolkit") | .name'
```

**Version source of truth:** `release-please-config.json` (components) and `.release-please-manifest.json` (current versions).

## Notes

- This repo uses multiple Go modules instead of one top-level module.
- Function packaging is validated on branch and pull request builds, then only published from function version tags.

Made with 🤓, 🐧 and 🍷.
