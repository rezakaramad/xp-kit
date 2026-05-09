# `xtenant-render` function call graph 

This function renders the desired tenant resources after approval. It builds Entra identity resources, waits for principal object IDs to appear in observed composed resources, renders ArgoCD Applications, bundles them into YAML, and writes that bundle through a GitHub `RepositoryFile` resource.

```mermaid
flowchart LR
    A[main.go\nCLI.Run] --> B[Load runtime config]
    A --> C[function.Serve]
    C --> D[fn.go\nRunFunction]

    D --> E[Load XR, desired,\nobserved, and input]
    E --> F[Approval gate]
    F --> G[Skip render if not approved]

    D --> H[fn-identity.go\nBuild principal resources]
    H --> I[Resolve principal object IDs]
    I --> J[Wait until IDs exist]

    D --> K[fn-baseline.go\nBuild baseline apps]
    D --> L[fn-gitops.go\nBuild gitops app]
    L --> L1[renderedRoles]
    L --> L2[generateAppRoleUUID]

    K --> M[fn-utils.go\nbundleYAML]
    L --> M
    M --> N[fn-repository-file.go\nBuild RepositoryFile]
    N --> O[response.SetDesiredComposedResources]
```

## Overview

- `main.go`: starts the render function, loads environment-based repository configuration, discovers the Crossplane namespace, and starts the gRPC server.
- `fn.go`: orchestration entry point. It loads observed and desired resources, parses the `XTenant`, handles the approval gate, coordinates identity resolution, renders ArgoCD applications, bundles YAML, and publishes the final `RepositoryFile` resource.
- `fn-identity.go`: builds Entra resources (user/group) for each binding and resolves principal object IDs from observed composed resources.
- `fn-gitops.go`: builds `gitOps-<TENANT>` Application, including roles, bindings, and deterministic app-role UUIDs.
- `fn-baseline.go`: builds `baseline-<TENANT>` Application per unique **destination cluster**.
- `fn-repository-file.go`: builds the GitHub `RepositoryFile` composed resource that writes the bundled YAML to the export repository.
- `fn-utils.go`: shared helpers for YAML bundling and deterministic UUID generation.
- `fn-types.go`: renderer-local types and shared metadata label helper.
- `input/v1beta1/input.go`: defines the function input schema, including GitHub config, Azure principal settings, and tenant bindings.
