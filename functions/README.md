# functions

This directory holds deployable Crossplane composition functions.

Use function packages for runtime behavior that will be built, packaged, and deployed into Crossplane.

## How to use it

- Put each function in its own directory.
- Keep function code, input types, package metadata, tests, and container build files together.
- Import shared logic from `modules/` instead of duplicating it across functions.

Example:

```sh
go test ./functions/xtenant-validate/...
```
