<p align="center" width="100%">
	<img width="24%" src="./logo.png">
</p>
<p align="center" >
	<img src="https://img.shields.io/badge/go-00ADD8?style=flat&logo=go&logoColor=white" />
  <img src="https://img.shields.io/badge/crossplane-326CE5?style=flat&logo=crossplane&logoColor=white" />
	<img src="https://img.shields.io/badge/release--please-34A853?style=flat&logo=google" />
	<img src="https://img.shields.io/badge/github%20actions-CI-2088FF?style=flat&logo=githubactions&logoColor=white" />
</p>

A workspace for building Crossplane-related libraries, functions, code generators, and shared API types.

This repository is organized as a small monorepo. Each top-level directory has a distinct role so reusable code, runnable function packages, CLI entrypoints, and shared type definitions can evolve independently.

## Repository structure

- `functions/`
	Contains deployable Crossplane composition functions.
	Each function directory is its own Go module and typically includes the function source code, input types, package metadata, container build files, and tests.

- `modules/`
	Contains reusable Go modules that support the rest of the repository.
	These modules are intended to hold shared libraries and utilities that can be consumed by functions, commands, or external projects.

- `cmd/`
	Contains executable entrypoints for repository-owned CLI tools.
	These commands are used for development workflows such as code generation or other standalone tooling.

- `types/`
	Contains shared API types and schemas.
	These packages define the resource models that other parts of the repository can build against.

- `.github/`
	Contains repository automation such as CI, release, and package publishing workflows.

## Layout conventions

- Each reusable library lives in its own Go module when it needs isolation or reuse. Only externally consumed surfaces need dedicated release management.
- Each deployable function keeps its runtime code, package metadata, and container build definition together in one directory.
- Shared types are separated from executable code so they can be imported without pulling in runtime concerns.
- Build and release automation is managed at the repository level, while function packaging assets live with each function.

## Typical workflow

1. Add or update shared types in `types/` when the resource model changes.
2. Put reusable logic in `modules/` when it should be shared across multiple binaries or functions.
3. Implement deployable function behavior in `functions/`.
4. Add command-line tooling in `cmd/` when repository workflows need a dedicated executable.

## Local checks

- VS Code workspace settings are configured to use `golangci-lint` on save.
- Run the same per-function checks used in CI with `./scripts/check-functions.sh <function-name>`.
- If you use `task`, the same entrypoints are available through `Taskfile.yml`.
- You can also run the matching VS Code tasks: `Check xtenant-validate`, `Check xtenant-render`, or `Check all functions`.

Examples:

```sh
./scripts/check-functions.sh xtenant-validate
./scripts/check-functions.sh xtenant-render
task check:xtenant-validate
task check:functions
```

## Notes

- This repository uses Go modules across multiple directories rather than a single top-level module.
- Function packaging is validated on branch and pull request builds, then published only from function version tags.

Made with 🤓, 🐧 and 🍷.
