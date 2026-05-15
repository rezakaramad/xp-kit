# How controller-tools and markers work

This note explains the two ideas that matter most when reading the generator code:

1. what a **marker** is
2. why the tool must **know which markers exist** before it can use them

## The short version

`controller-tools` reads your Go types and turns them into Kubernetes API metadata such as schemas.

A **marker** is a special comment that adds extra instructions to your Go code.

Example:

```go
// +kubebuilder:validation:Minimum=1
// +kubebuilder:default=3
Replicas int32 `json:"replicas,omitempty"`
```

These comments are not normal Go syntax rules. They are instructions for tools like `controller-tools`.

Without markers, Go can tell you that `Replicas` is an `int32`.
With markers, the tool can also learn that:

- the value must be at least `1`
- the default value is `3`

## What a marker is

A marker is a structured comment, usually starting with `+kubebuilder:`.

Examples:

```go
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Enum=small;medium;large
// +kubebuilder:default=1
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
```

In plain language, markers let you add metadata that normal Go types do not express by themselves.

Go types can describe:

- this field is a `string`
- this field is an `int32`
- this field is a slice
- this field is a nested struct

Markers can describe extra API rules like:

- minimum or maximum values
- allowed enum values
- default values
- whether the type is a root Kubernetes object
- whether status is a subresource

## Why controller-tools needs markers

When `controller-tools` generates a schema, it combines three things:

1. the Go type itself
2. the JSON tags
3. the markers

For example:

```go
type XAppSpec struct {
    // +kubebuilder:validation:MinLength=1
    Name string `json:"name"`

    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:default=1
    Replicas int32 `json:"replicas,omitempty"`
}
```

From this, the tool can infer:

- `name` is a string
- `name` must be at least 1 character long
- `replicas` is an integer
- `replicas` must be at least 1
- `replicas` defaults to 1

That becomes part of the generated OpenAPI schema.

## Why the tool must know which markers exist

This is the key point.

A marker is just text in a comment until the tool is taught what it means.

For example, this line:

```go
// +kubebuilder:validation:Minimum=1
```

looks meaningful to us, but a program cannot safely use it unless it already knows:

- that `+kubebuilder:validation:Minimum` is a valid marker name
- where that marker is allowed to appear
- what kind of value it expects
- how to parse the value `1`
- what effect it should have on the generated schema

Without that knowledge, the tool has only two bad choices:

- ignore the comment
- guess what it means

Ignoring it would lose validation behavior.
Guessing would be unreliable.

That is why `controller-tools` has to register marker definitions first.

## The simple analogy

Think of it like this:

- marker comments are words in a language
- the registry is the dictionary
- the parser is the reader

If the reader sees a word that is not in the dictionary, it cannot know what that word means.

## What these two lines do

In the generator code you have:

```go
reg := &markers.Registry{}
gen := crd.Generator{}
```

These two lines are setting up the marker system.

### `reg := &markers.Registry{}`

This creates an empty marker registry.

In simple terms, it is:

> a place where known marker definitions will be stored

At this point, the registry is empty. It does not yet know what `+kubebuilder:validation:Minimum` means.

### `gen := crd.Generator{}`

This creates the controller-tools CRD generator value.

In simple terms, it is:

> the component that knows which CRD-related markers should exist

By itself, it does not change anything yet.

The important next step is:

```go
if err := gen.RegisterMarkers(reg); err != nil {
    return nil, fmt.Errorf("registering markers: %w", err)
}
```

This means:

> ask the CRD generator to register all the marker definitions it knows into the registry

After that, the registry understands the kubebuilder markers needed for schema generation.

## Simple example

Imagine you have this Go type:

```go
type AppSpec struct {
    // +kubebuilder:validation:Minimum=1
    // +kubebuilder:default=3
    Replicas int32 `json:"replicas,omitempty"`

    // +kubebuilder:validation:Enum=dev;staging;prod
    Environment string `json:"environment"`
}
```

Now think of the marker setup flow like this.

### Step 1: Create an empty registry

```go
registry := &markers.Registry{}
```

At this point, the registry knows nothing.

If the parser sees:

```go
// +kubebuilder:validation:Minimum=1
```

it still does not know what that marker means.

### Step 2: Create the CRD generator helper

```go
generator := crd.Generator{}
```

This `generator` value knows the standard CRD and kubebuilder marker definitions that controller-tools supports.

For example, it knows about marker families such as:

- validation markers
- default markers
- object/root markers
- subresource markers

### Step 3: Register those marker definitions

```go
err := generator.RegisterMarkers(registry)
```

This means:

> copy the known CRD marker definitions into the registry

After this, the registry can understand comments like:

```go
// +kubebuilder:validation:Minimum=1
// +kubebuilder:default=3
// +kubebuilder:validation:Enum=dev;staging;prod
```

### Step 4: Let the parser use the registry

Later, when the parser reads your type, it can turn those comments into schema rules like:

- `replicas.minimum = 1`
- `replicas.default = 3`
- `environment.enum = ["dev", "staging", "prod"]`

That is why `RegisterMarkers` matters: it makes marker comments understandable before parsing begins.

## The basic terms you should know

| Term | Meaning |
|---|---|
| Go type | A struct, field, alias, or other Go type definition |
| JSON tag | The serialized field name, e.g. ``json:"name,omitempty"`` |
| Marker | A special `// +kubebuilder:...` comment understood by tooling |
| Registry | A catalog of known marker definitions |
| Collector | The part that scans comments and extracts markers |
| Parser | The part that reads types and markers and builds schema information |
| Schema | The machine-readable description of fields, types, and validation rules |
| CRD | Kubernetes CustomResourceDefinition |
| OpenAPI schema | The validation schema embedded in a CRD or XRD |

## How this fits in your generator

The flow in the generator is:

1. load the Go package
2. create an empty marker registry
3. register the known CRD/kubebuilder markers
4. create a parser that uses that registry
5. parse the target type
6. generate the schema

So the registry setup is not optional plumbing. It is what makes marker comments meaningful to the tool.

## One sentence summary

Markers are structured comments that describe API behavior, and `controller-tools` must register their definitions first so it can parse them correctly and turn them into schema.
