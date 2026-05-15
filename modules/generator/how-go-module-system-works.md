# How the Go module system works

This note explains the basic ideas behind Go modules, package paths, and why code like this exists:

```go
config := &packages.Config{Dir: moduleDir}
```

If Go's module system feels confusing at first, that is normal. The main difficulty is that Go works with both **import paths** and **real directories on disk**, and the code has to connect those two worlds.

## The short version

When the generator gets a package path like:

```go
github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple
```

that is **not** a filesystem path.

It is an **import path**.

Before Go can load and inspect that package, it has to figure out:

- where the package lives on disk
- which `go.mod` file controls it
- which dependency versions apply
- whether any `replace` directives change where code comes from

That is why the generator first resolves the package directory, then tells the Go loader to use that directory as its working context.

## Three different things

These three are easy to mix up:

| Thing | Example | Meaning |
|---|---|---|
| Module path | `github.com/rezakaramad/crossplane-toolkit/modules/generator` | The name declared in `go.mod` |
| Package path | `github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple` | The import path for a specific package |
| Filesystem path | `/home/kara/github/r-karamad/crossplane-toolkit/modules/generator/testdata/xsimple` | The actual directory on disk |

A package path usually starts with the module path, then adds subdirectories under it.

## How the filesystem path is built

Suppose:

- the module path in `go.mod` is `github.com/rezakaramad/crossplane-toolkit/modules/generator`
- the module root on disk is `/home/kara/github/r-karamad/crossplane-toolkit/modules/generator`
- the package path is `github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple`

Go resolves that package like this:

1. Start with the full package path.
2. Remove the module path prefix.
3. Append the remaining subpath to the module root directory.

That looks like this:

```text
package path: github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple
module path:  github.com/rezakaramad/crossplane-toolkit/modules/generator
subpath:      /testdata/xsimple

module root:  /home/kara/github/r-karamad/crossplane-toolkit/modules/generator
final path:   /home/kara/github/r-karamad/crossplane-toolkit/modules/generator/testdata/xsimple
```

So yes, the basic idea is:

- find the module root on disk
- strip the module path prefix from the package path
- append the rest to the module root

## What `go.mod` does

A Go module is usually a directory tree with a `go.mod` file at its root.

Example:

```go
module github.com/rezakaramad/crossplane-toolkit/modules/generator
```

That tells Go:

> This directory is the root of the module, and packages under it use this import path prefix.

So if the module root directory is:

```text
/home/kara/github/r-karamad/crossplane-toolkit/modules/generator
```

and inside it you have:

```text
testdata/xsimple/types.go
```

then the package path becomes:

```go
github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple
```

## How Go finds dependencies

When your code imports another package, Go looks at the active module and its `go.mod` file.

That is how it decides:

- which version of each dependency to use
- whether the dependency should come from the module cache
- whether a `replace` directive points to a local path instead

For example, a `replace` can say:

```go
replace github.com/example/lib => ../local-lib
```

That means:

> Do not use the downloaded copy from the internet. Use the local directory instead.

This is why the correct module context matters. The same import path can resolve differently depending on the active `go.mod`.

## Why `findModuleDir` exists

The generator starts with a package path like:

```go
github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple
```

But `controller-tools` and the Go loader need a real directory on disk.

So `findModuleDir` translates from:

- package path

to:

- filesystem path

It does that by checking a few places:

1. the current binary's own module
2. dependency modules embedded in build info
3. `replace` rules in `go.mod`
4. required modules in the Go module cache

## What this line means

```go
config := &packages.Config{Dir: moduleDir}
```

This tells the Go package loader:

> Load the package using `moduleDir` as the working directory.

In plain language:

> Start from this folder when figuring out which module rules apply.

That matters because Go needs the correct module context to resolve:

- imports
- `replace` directives
- dependency versions
- the active `go.mod`

Without this, the loader may:

- fail to find the package
- use the wrong module
- ignore a local replacement
- load dependencies incorrectly

## A simple analogy

Think of a package path like a mailing address written in a company directory, not on a city map.

The generator first asks:

> Where is this address physically located?

That is what `findModuleDir` does.

Then it tells Go:

> Start your search from this building.

That is what this does:

```go
config := &packages.Config{Dir: moduleDir}
```

## How it fits in the generator

The overall flow is:

1. user gives a package path and struct name
2. `findModuleDir` finds the package on disk
3. `packages.Config{Dir: moduleDir}` gives Go the right module context
4. `loader.LoadRootsWithConfig(...)` loads the package
5. `controller-tools` reads the struct fields and kubebuilder markers
6. the generator turns that into an OpenAPI schema

## Why this matters for the XRD generator

Without this machinery, users would have to manually write OpenAPI schema YAML by hand.

This module lets you:

- define the resource in Go
- add validation with kubebuilder markers
- generate the schema automatically

That keeps the Go type and the schema in sync and reduces manual mistakes.

## One sentence summary

The Go module system maps import paths to real code on disk, and this generator uses that mapping so it can find your Go types, load them correctly, and turn them into XRD schema.
