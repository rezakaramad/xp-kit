# gen-xrd

A CLI tool that generates Crossplane `CompositeResourceDefinition` YAML from annotated Go structs.

Pass the Go import path of any package containing a kubebuilder-annotated XR type and get apply-ready XRD YAML in return. No code changes are needed when you add a new XR type — just run the command.

## Usage

```sh
go run github.com/rezakaramad/crossplane-toolkit/cmd/gen-xrd \
  --package github.com/rezakaramad/crossplane-toolkit/types/xtenant \
  --type    XTenant \
  --group   idp.rezakara.demo \
  --version v1beta1 \
  --output  xtenant-xrd.yaml
```

Omit `--output` to print to stdout and pipe directly into kubectl:

```sh
go run github.com/rezakaramad/crossplane-toolkit/cmd/gen-xrd \
  --package github.com/rezakaramad/crossplane-toolkit/types/xtenant \
  --type    XTenant \
  --group   idp.rezakara.demo \
  --version v1beta1 \
  | kubectl apply -f -
```

## Flags

| Flag | Required | Default | Description |
|---|---|---|---|
| `--package` | yes | — | Go import path of the package containing the XR type |
| `--type` | yes | — | Struct name of the root composite resource |
| `--group` | yes | — | Crossplane API group, e.g. `idp.rezakara.demo` |
| `--version` | no | `v1alpha1` | API version |
| `--plural` | no | kind + `s` | Override the plural resource name |
| `--output` | no | stdout | Write YAML to this file instead of stdout |

## Working with local (unpublished) type modules

If your XR types module is not yet published, add a `replace` directive to this module's `go.mod`:

```
replace github.com/rezakaramad/crossplane-toolkit/types/xtenant => ../../types/xtenant
```

Then run:

```sh
go get github.com/rezakaramad/crossplane-toolkit/types/xtenant
go mod tidy
```

## How it works

`gen-xrd` uses `modules/generator` to do the actual XRD generation:

1. Resolves the type package from the module cache or a `replace` directive.
2. Parses the Go source code structure and collects `+kubebuilder:validation:*` and `+kubebuilder:default` markers.
3. Flattens the resulting OpenAPI schema (removes `$ref` / `allOf` wrappers).
4. Wraps the schema in a `CompositeResourceDefinition` object and serialises it to YAML, omitting the `status` field.
