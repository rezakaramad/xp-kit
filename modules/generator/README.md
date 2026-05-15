# generator

Core library for generating Crossplane `CompositeResourceDefinition` manifests from Go struct definitions.

It takes a Go package path and type name, parses the struct fields and kubebuilder marker annotations using [controller-tools](https://github.com/kubernetes-sigs/controller-tools), and returns a fully populated `CompositeResourceDefinition` object ready to be marshalled to YAML.

## Installation

```sh
go get github.com/rezakaramad/crossplane-toolkit/modules/generator
```

## Usage

```go
import "github.com/rezakaramad/crossplane-toolkit/modules/generator"

xrd, err := generator.BuildCompositeResourceDefinition(generator.ResourceMeta{
    PackagePath: "github.com/yourorg/yourrepo/resources/xdeployment",
    TypeName:    "XDeployment",
    Group:       "platform.example.org",
    Version:     "v1alpha1", // optional, defaults to v1alpha1
})
if err != nil {
    log.Fatal(err)
}

out, err := generator.MarshalXRDToYAML(xrd)
if err != nil {
    log.Fatal(err)
}

os.Stdout.Write(out)
```

## API surface

| Symbol | Description |
|---|---|
| `ResourceMeta` | Input: package path, type name, group, and version |
| `BuildCompositeResourceDefinition` | Parses the package and returns a `*CompositeResourceDefinition` |
| `ExtractOpenAPISchema` | Lower-level: returns the raw flattened OpenAPI v3 schema for a type |
| `MarshalXRDToYAML` | Renders the XRD to apply-ready YAML, omitting the `status` field |

## How it works

1. **Locate source** — resolves the package's on-disk directory from build info, `go.mod` replace directives, or the module cache.
2. **Parse & collect markers** — uses controller-tools to walk the AST and collect `+kubebuilder:validation:*` and `+kubebuilder:default` markers.
3. **Flatten schema** — resolves all `$ref` pointers and removes `allOf` wrappers so the schema can be embedded directly in the XRD.
4. **Wrap in XRD** — places the `spec` (and optionally `status`) schema under a top-level object and populates all required XRD fields.
