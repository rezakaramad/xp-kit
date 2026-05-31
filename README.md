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

## How function publishing works

Each function produces two artifacts: a **runtime image** (a regular Docker image with the Go binary) and a **Crossplane package** (an `.xpkg` file that embeds the runtime image and is what you install on a cluster). Understanding both is necessary to add a new function or debug a publish failure.

### The two artifacts

| Artifact | Registry path | What it is |
| --- | --- | --- |
| Runtime image | `ghcr.io/<owner>/<name>-runtime:<version>` | A distroless container image built from the function's `Dockerfile`. Contains only the compiled Go binary. Never installed directly — it is embedded inside the Crossplane package. |
| Crossplane package | `ghcr.io/<owner>/function-<name>:<version>` | An `.xpkg` OCI artifact built by `crossplane xpkg build`. It bundles the runtime image together with the function metadata from `package/crossplane.yaml`. This is what you reference in a `Function` resource on the cluster. |

### What each file in a function directory does

```
functions/xtenant-validate/
├── Dockerfile          # Builds the runtime image (Go binary in distroless)
├── package/
│   └── crossplane.yaml # Function metadata — name, capabilities. One file, no schema needed.
├── go.mod              # The function's own Go module
└── *.go                # Function logic
```

**`Dockerfile`** — two-stage build: `golang` image compiles the binary, `distroless/static` runs it. The build workflow passes `GO_VERSION` and target platform as build args.

**`package/crossplane.yaml`** — declares the function to Crossplane. Minimum viable content:

```yaml
apiVersion: meta.pkg.crossplane.io/v1beta1
kind: Function
metadata:
  name: function-<name>
spec:
  capabilities:
    - composition
```

The `name` here must match what you reference in `Composition` resources (`functionRef.name`).

### How the build workflow assembles them

1. **Build runtime image** — `docker build` from the function's `Dockerfile`, tagged as `...-runtime:<version>`
2. **Push runtime image** — only on tag-triggered or dispatched runs
3. **Build Crossplane package** — `crossplane xpkg build --package-root package/ --embed-runtime-image ...-runtime:<version>` produces `<name>.xpkg`
4. **Push Crossplane package** — `crossplane xpkg push` uploads the `.xpkg` to GHCR as `function-<name>:<version>` and also as `latest`

The runtime image must be built and available in the local Docker daemon before `xpkg build` runs, because the embed step pulls it from there.

### Adding a new function

1. Create `functions/<name>/` with a `Dockerfile`, `package/crossplane.yaml`, `go.mod`, and your Go code.
2. Add it to `go.work` under the `use` block.
3. Add it to `release-please-config.json` so release-please tracks its version.
4. Add `<name>: 0.0.0` to `.release-please-manifest.json` as the initial baseline.
5. The build workflow auto-discovers all directories under `functions/` — no workflow changes needed.

### Verifying published packages

```sh
# List all GHCR packages from this repo
gh api '/users/rezakaramad/packages?package_type=container&per_page=100' \
  --jq '.[] | select(.repository.full_name == "rezakaramad/crossplane-toolkit") | .name'

# Inspect versions of a specific package
gh api '/users/rezakaramad/packages/container/function-xtenant-validate/versions' \
  --jq '.[] | [.id, .metadata.container.tags[]] | @tsv'
```

## Release flow

There are two moving parts:

- [release-please](https://github.com/googleapis/release-please) watches `main`, bumps changelogs, and creates GitHub Release entries for the configured components.
- Tags trigger the actual publish workflows. Without a tag, nothing is published.

### Component types and what their tags do

| Component | Tag format | What the tag publishes |
| --- | --- | --- |
| `functions/*` | `functions/<name>/v<semver>` | GHCR function package + runtime image |
| `cmd/*` | `cmd/<name>/v<semver>` | Binary archives + checksums attached to the GitHub Release |
| `modules/*`, `types/*` | `modules/<name>/v<semver>`, `types/<name>/v<semver>` | The semver tag itself — no binary or OCI artifact; Go module consumers resolve directly from the tag |

### Tag ordering rules

The repo is a monorepo where functions depend on shared libraries (`modules/*`, `types/*`). The **Go module proxy caches versions immutably**: once it fetches a tag, it keeps that snapshot forever. This means:

- **Never delete and recreate a version tag.** The proxy will never pick up the new content. Bump the patch version instead.
- **Tag dependencies before dependents.** Shared libraries must be tagged and proxy-indexed before function Docker release builds can resolve them. The order is:

```
types/* → modules/* → functions/* (and cmd/*)
```

If you tag a function before its library deps are proxy-indexed, the function's Docker build will compile against stale cached code.

> CI tests (lint, unit-test) are not affected by this ordering because `go.work` is committed and CI resolves all workspace modules from the local checkout, bypassing the proxy entirely.

### Step-by-step release sequence

**1. Develop on a feature branch**

```sh
git checkout -b feat/my-change
# ... make changes across types/, modules/, functions/ ...
git push origin feat/my-change
```

**2. Update `go.mod` references before merging**

If you bumped an API in a shared library that a function depends on, you need to reference the *upcoming* version in the function's `go.mod` before merging. The sequence is:

```sh
# a. Decide the new version for the library (e.g. types/xtenant v0.2.0)
# b. Update the function's go.mod to require that version
# c. Run go mod tidy in the function directory
# d. Commit everything — go.mod, go.sum — on the feature branch
```

The library tag does not need to exist yet at this point. CI will compile fine because `go.work` resolves the library from the local tree.

**3. Open a PR, review, and merge into `main`**

**4. Let release-please open the release PR**

release-please reads conventional commits and bumps the version for each affected component in `.release-please-manifest.json`. It opens (or updates) a single release PR that touches all changed components at once.

> If release-please misses a component, check that your commit scope matches the package path exactly.
> For example `feat!(types/xtenant):` not `feat!(xtenant):`.

**5. Merge the release PR**

Merging the release PR does two things automatically: it updates `.release-please-manifest.json` on `main` and creates the GitHub Release entries. Tags are **not** created by release-please in this repo — you create them manually in step 6.

**6. Tag and push shared libraries first**

After the release PR is merged, `main` is the right base. Tag libraries in dependency order:

```sh
git pull
git tag types/xtenant/v0.2.0
git tag modules/nextinsight/v0.2.0
git push origin types/xtenant/v0.2.0 modules/nextinsight/v0.2.0
```

Wait a few seconds for the Go module proxy to index the new tags:

```sh
curl -s "https://proxy.golang.org/github.com/rezakaramad/crossplane-toolkit/types/xtenant/@v/v0.2.0.info"
curl -s "https://proxy.golang.org/github.com/rezakaramad/crossplane-toolkit/modules/nextinsight/@v/v0.2.0.info"
```

Both should return a JSON object with a `"Version"` field before you proceed.

**7. Tag functions (and CLI tools) last**

```sh
git tag functions/xtenant-render/v0.1.0
git tag functions/xtenant-validate/v0.1.0
git push origin functions/xtenant-render/v0.1.0 functions/xtenant-validate/v0.1.0
```

Each function tag triggers the build workflow, which builds the Docker runtime image and publishes the Crossplane function package to GHCR.

> **Note — release-please tags do not trigger workflows.**
> When release-please merges its PR it creates the version tags automatically using `GITHUB_TOKEN`. GitHub intentionally prevents workflows from being triggered by `GITHUB_TOKEN` events to avoid infinite loops, so those tag pushes are silently ignored by the build workflow. You must push the tags yourself (step 7 above) for the publish to happen.
>
> If you find the tags already exist on the remote and `git push` says "Everything up-to-date", use `gh workflow run` to dispatch the build manually:
>
> ```sh
> gh workflow run "Build and publish Crossplane function packages" --ref functions/xtenant-render/v0.1.0
> gh workflow run "Build and publish Crossplane function packages" --ref functions/xtenant-validate/v0.1.0
> ```

### Letting Copilot execute this flow

You can ask Copilot to perform the release instead of running the steps manually. It will read `.release-please-manifest.json` and recent `git log` to determine what changed and which versions to bump, check for an open or already-merged release PR, and run the tagging sequence in the right order including proxy verification.

The only input it needs from you is a go-ahead on the release PR:

> *"The release PR is merged — do the release"*

or, if you want it to manage the PR too:

> *"Open the release PR, wait for my approval, then tag everything"*

Everything else — current versions, changed components, tag order, proxy verification — it can determine from the repo state.

### Version source of truth

- `release-please-config.json` — which components participate in automated releases.
- `.release-please-manifest.json` — the current released version for each component. If you need to reset a component to a new baseline, update this file before creating new tags.

### What ends up where

- GitHub Releases — created by release-please after the release PR is merged.
- GHCR function packages — created by the tag-triggered build workflow (`functions/<name>/v*`).
- Go module tags — the semver tag itself; no workflow needed.
- Function package files (`.xpkg`) are **not** attached to the GitHub Release; the installable artifacts live in GHCR.

When you look at GitHub Packages you will see both `function-...` and `...-runtime` images:

- `function-...` is the Crossplane function package you install from a `Function` resource.
- `...-runtime` is the backing container image used by that package.
- In normal usage, you want `function-...`, not `...-runtime`.

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
