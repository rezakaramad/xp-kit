# runner

This package helps structure a Crossplane composition function around typed builders.

Use it when you want one function to manage multiple composed resources without putting all of the decode, observe, build, and response logic in a single `RunFunction` implementation.

## How To Use It

- Create a typed runner with your XR type and your function input type.
- Implement one `Builder` per composed resource you want the function to manage.
- Register those builders in `RunFunction`, then let the runner execute the flow.

Example:

```go
r := runner.New[*MyXR, *MyInput](req, log)

runner.Register(r, deploymentBuilder{})
runner.Register(r, serviceBuilder{})

return r.Run(ctx)
```

Each builder is responsible for one resource and implements:

- `Condition()`
- `ResourceName(...)`
- `Skip(...)`
- `Desired(...)`
- `Ready(...)`
- `Connection(...)`

That keeps the function entrypoint small while still giving each resource its own typed logic.