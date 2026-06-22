package runner

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/function-sdk-go/logging"
	"github.com/crossplane/function-sdk-go/resource"
)

// Context is passed to every Composer method on each reconcile.
// It gives you everything you need to build a child resource:
//   - Ctx:      Go context for cancellation and deadlines (pass to external calls like HTTP/gRPC)
//   - XR:       the parent composite resource being reconciled (read its spec to know what to build)
//   - Input:    the function input defined in the Composition (shared config like image registries, regions, etc.)
//   - Observed: all child resources that already exist in the cluster, keyed by their stable name.
//     Use ObservedAs to read a sibling child's status — for example to get a generated ID from one child and pass it to another.
//   - Log:      a structured logger tagged with the XR name, ready to use
type Context[XR any, Input runtime.Object] struct {
	Ctx      context.Context
	XR       XR
	Input    Input
	Observed map[resource.Name]resource.ObservedComposed
	Log      logging.Logger
}

// ObservedAs decodes an existing child resource from ctx.Observed into type T.
// Returns a nil T when the resource does not exist yet - not an error (e.g. on first reconciliation).
func ObservedAs[T runtime.Object](
	observed map[resource.Name]resource.ObservedComposed,
	resourceName resource.Name,
) (T, error) {
	if obs, exists := observed[resourceName]; exists {
		output := newDecodeTarget[T]()
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
			obs.Resource.UnstructuredContent(), output,
		); err != nil {
			var zero T
			return zero, err
		}
		return output, nil
	}
	var zero T
	return zero, nil
}
