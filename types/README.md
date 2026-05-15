# types

This directory holds shared Go type packages for composite resources.

Use these packages as the source of truth for resource shape, validation markers, and generated XRD schema.

## How to use it

- Put each resource type in its own package directory.
- Keep kubebuilder markers close to the Go structs they describe.
- Generate XRD YAML from a type package with `cmd/gen-xrd`.

Example:

```sh
go run github.com/rezakaramad/crossplane-toolkit/cmd/gen-xrd \
  --package github.com/rezakaramad/crossplane-toolkit/types/xtenant \
  --type XTenant \
  --group idp.rezakara.demo \
  --version v1beta1
```
