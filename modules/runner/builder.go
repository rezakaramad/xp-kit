package runner

import (
	"github.com/crossplane/function-sdk-go/logging"
	"github.com/crossplane/function-sdk-go/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// MetadataLabeler is implemented by any type that can produce Kubernetes-safe
// labels. [AppMetadata] from the nextinsight module satisfies this interface,
// but callers can provide their own implementation.
type MetadataLabeler interface {
	Labels() map[string]string
}

// Context is the common input that every builder gets while the function runs.
// In this case,
//
//	One builder is equal to one resource that we want to manage (e.g. a ConfigMap or a Secret).
//	It knows how to create that resource
//	It knows how to check whether that resource is ready
//	It knows how to extract connection details from that resource if needed
//
// A simple way to think about it is:
// each resource-specific type, like ServiceBuilder, DeploymentBuilder, implements the Builder interface
// and receives the same Context.
// Each one then uses that shared ctx to build its own resource.

// The Context contains the following fields:
//   - XR:          the main composite resource we are working on
//   - Defaults:    the input/default values for this function step
//   - Log:         a logger for writing debug or info messages
//   - AppMetadata: optional metadata fetched from an external source (e.g. Next-Insight).
//     When non-nil, builders should call ctx.StampMetadata(obj) to apply labels
//     and annotations onto each composed child resource.
type Context[XR any, Defaults any] struct {
	XR          XR
	Defaults    Defaults
	Log         logging.Logger
	AppMetadata MetadataLabeler
}

// StampMetadata merges labels from ctx.AppMetadata onto obj.
// It is a no-op when ctx.AppMetadata is nil, so builders can call it
// unconditionally without guarding on whether metadata was resolved.
func (c Context[XR, Defaults]) StampMetadata(obj metav1.Object) {
	if c.AppMetadata == nil {
		return
	}

	// Merge labels — metadata keys win on conflict because the metadata source
	// (e.g. Next-Insight) is the system of record.
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range c.AppMetadata.Labels() {
		labels[k] = v
	}
	obj.SetLabels(labels)
}

// Builder is the interface for one resource-specific worker.
// It responds:
// - What resources am I responsible for? (ResourceName)
// - Should I skip this resource? (Skip)
// - What does the desired resource look like? (Desired)
// - Is the observed resource ready? (Ready)
// - What connection details can I extract from the observed resource? (Connection)
// Type parameters:
//   - XR       the typed composite resource pointer (e.g. *XDeployment)
//   - Defaults the typed Composition step input pointer (e.g. *XDeploymentDefaults)
//   - Observed the observed Kubernetes resource pointer (e.g. *corev1.Service)
//   - Desired  the desired Kubernetes resource pointer (e.g. *corev1.Service)
//
// E.g., A ServiceBuilder would be Builder[*XDeployment, *XDeploymentDefaults, *corev1.Service, *corev1.Service]
// Each builder follow the same shape:
// 	- Condition() string: returns the condition type that this builder manages (e.g. "ServiceReady")
// 	- ResourceName(ctx Context[XR, Defaults]) resource.Name: returns the name of the resource this builder manages. This is used to correlate observed and desired resources.
// 	- Skip(ctx Context[XR, Defaults]) bool: returns true if this builder should be skipped. This allows conditional logic to determine whether a resource should be managed or not.
// 	- Desired(ctx Context[XR, Defaults]) (Desired, error): builds the desired resource based on the input context. This is where you define what the child resource should look like.
// 	- Ready(ctx Context[XR, Defaults], observed Observed) bool: checks if the observed resource is ready. This is where you define the logic to determine if the child resource is in a ready state.
// 	- Connection(ctx Context[XR, Defaults], observed Observed) map[string]string: extracts connection details from the observed resource. This is where you define how to get connection information (like endpoints, credentials, etc.) from the child resource.

type Builder[XR any, Defaults any, Observed runtime.Object, Desired runtime.Object] interface {
	// Condition returns the XR status condition type for this resource.
	// The string is reported directly on the composite resource status.
	// Examples: "DeploymentReady", "ServiceReady", "HTTPRouteReady".
	Condition() string

	// ResourceName returns the stable Crossplane composition resource name used
	// to track this resource across reconciliations. The name must be unique
	// within the function and stable for a given XR name.
	// Example: "xdeployment-service-" + ctx.XR.GetName()
	ResourceName(ctx Context[XR, Defaults]) resource.Name

	// Skip allows a builder to be conditionally skipped. This is useful when the presence of a resource is optional based on the input or observed state.
	// E.g., if ingress is disabled in the input, we can skip building the HTTPRoute resource.
	Skip(ctx Context[XR, Defaults]) bool

	// Desired builds the desired resource based on the input context.
	// This is where you define what the child resource should look like.
	Desired(ctx Context[XR, Defaults]) (Desired, error)

	// Ready checks if the observed resource is ready.
	// This is where you define the logic to determine if the child resource is in a ready state.
	Ready(ctx Context[XR, Defaults], observed Observed) bool

	// Connection extracts connection details from the observed resource.
	// This is where you define how to get connection information
	// (like host, port, URL, connection string, etc.) from the child resource.
	Connection(ctx Context[XR, Defaults], observed Observed) map[string]string
}
