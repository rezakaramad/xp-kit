# modules

This directory holds reusable Go modules shared across the repository.

Use modules for logic that should not live inside a single function or command.

## How to use it

- Put shared libraries in their own module directory.
- Keep executable entrypoints out of this directory.
- Import these modules from `functions/`, `cmd/`, or other Go projects.

Example:

```sh
go get github.com/rezakaramad/crossplane-toolkit/modules/generator
```
