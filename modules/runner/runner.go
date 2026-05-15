package runner

import (
	"context"
	"fmt"
	"reflect"

	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/response"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// init runs once automatically when this package is loaded by Go.
//
// Here we register core Kubernetes types like Secret, ConfigMap, and Service
// into Crossplane's composed scheme. This is needed so composed.From(...)
// knows how to convert those normal Go objects into composed resources.
//
// Without this registration, converting a core/v1 object can fail at runtime
// with an error saying its kind is not registered in the scheme.
func init() {
	if err := corev1.AddToScheme(composed.Scheme); err != nil {
		panic(fmt.Sprintf("register corev1 types in composed scheme: %v", err))
	}
}

// Runner owns the full composition function lifecycle for one pipeline step.
// It decodes the XR, decodes the step input, reads observed and desired state,
// processes each registered Builder, aggregates connection details, and writes
// the final desired composed resources back into the response.
//
// XR must be a pointer to the typed composite resource struct (e.g. *XDeployment).
// D  must be a pointer to the typed Composition step input struct (e.g. *XDeploymentDefaults).
//
// Typical usage inside RunFunction:
//
//	r := runner.New[*MyXR, *MyDefaults](req, f.log)
//	runner.Register(r, myDeploymentBuilder{})
//	runner.Register(r, myServiceBuilder{})
//	return r.Run(ctx)
type Runner[XR any, D any] struct {
	req      *fnv1.RunFunctionRequest
	log      logging.Logger
	builders []internalBuilder[XR, D]
}

// New creates a Runner for the given request and logger.
// Typically called once at the top of a RunFunction invocation.
func New[XR any, D any](req *fnv1.RunFunctionRequest, log logging.Logger) *Runner[XR, D] {
	return &Runner[XR, D]{req: req, log: log}
}

// Register adds a typed Builder to the Runner.
//
// This is a package-level function because Go methods cannot introduce new type
// parameters. Builders are processed in the order they are registered.
//
//	r := runner.New[*MyXR, *MyDefaults](req, log)
//	runner.Register(r, myDeploymentBuilder{})
//	runner.Register(r, myServiceBuilder{})
//	return r.Run(ctx)
func Register[XR any, Defaults any, Observed runtime.Object, Desired runtime.Object](
	runner *Runner[XR, Defaults],
	builder Builder[XR, Defaults, Observed, Desired]) {
	// Wrap the typed builder in the adapter layer used by Runner.
	runner.builders = append(runner.builders, &builderAdapter[XR, Defaults, Observed, Desired]{builder: builder})
}

// Run executes the full function flow.
//
// Composite resource is the parent XR being reconciled.
// Composed resources are the child resources managed for that XR.
//
// The runner reads the observed parent XR and observed child resources,
// lets builders produce the desired child resources, and writes the final
// desired composed resources back into the function response.
func (runner *Runner[XR, D]) Run(_ context.Context) (*fnv1.RunFunctionResponse, error) {
	rsp := response.To(runner.req, response.DefaultTTL)

	// Get the observed composite resource (parent XR) from the request and decode it into a typed XR struct.
	observedCompositeResource, err := request.GetObservedCompositeResource(runner.req)
	if err != nil {
		runner.fatal(rsp, "cannot get observed composite resource", err)
		return rsp, nil
	}

	// Create an empty typed XR object, then copy the observed parent XR data from the request into it.
	xr := newDecodeTarget[XR]()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		observedCompositeResource.Resource.UnstructuredContent(), xr,
	); err != nil {
		runner.fatal(rsp, "cannot decode composite resource", err)
		return rsp, nil
	}

	// The input data is what Crossplane sends in the request
	// based on the composition step's input configuration.
	// We decode it into a typed Defaults struct that builders can use.
	defaults := newDecodeTarget[D]()
	inputObject, ok := any(defaults).(runtime.Object)
	if !ok {
		runner.fatal(rsp, "function input does not implement runtime.Object", fmt.Errorf("input type must implement runtime.Object"))
		return rsp, nil
	}
	if err := request.GetInput(runner.req, inputObject); err != nil {
		runner.fatal(rsp, "cannot get function input", err)
		return rsp, nil
	}

	// Get the observed composed resources (child resources) from the request.
	observedComposedResources, err := request.GetObservedComposedResources(runner.req)
	if err != nil {
		runner.fatal(rsp, "cannot get observed composed resources", err)
		return rsp, nil
	}

	// Get the desired composed resources (child resources) from the request.
	desired, err := request.GetDesiredComposedResources(runner.req)
	if err != nil {
		runner.fatal(rsp, "cannot get desired composed resources", err)
		return rsp, nil
	}

	// We create a shared Context object that is passed to all builders.
	// This contains the decoded XR, the input defaults, and a logger.
	// We log which XR we are working on
	// And later we can add child resource-specific information to the log context as we loop through builders.
	log := runner.log.WithValues(
		"xr-version", observedCompositeResource.Resource.GetAPIVersion(),
		"xr-kind", observedCompositeResource.Resource.GetKind(),
		"xr-name", observedCompositeResource.Resource.GetName(),
	)

	// The context is shared across all builders, so it contains the common information they all need.
	functionContext := Context[XR, D]{
		XR:       xr,
		Defaults: defaults,
		Log:      log,
	}

	// Contains all connection data we want to publish for this XR, collected from builders.
	// E.g., one build might return
	// map[string]string{
	//     "host": "postgres.default.svc.cluster.local",
	//     "port": "5432",
	// }
	connectionDetails := map[string]string{}

	// The runner goes through each builder one by one and asks:
	// - What child resource do you want, and is it ready yet? (Desired and Ready)
	// - Do you have any connection details I should know about? (Connection)
	// The runner is just the coordinator that runs the builders in sequence
	// and collects their results.
	// One builder represents one child resource.
	// E.g., a ServiceBuilder manages the desired state of a Service child resource,
	// and knows how to check if it's ready, and what connection details to extract from it.
	for _, b := range runner.builders {
		result, err := b.process(functionContext, observedComposedResources)
		if err != nil {
			response.ConditionFalse(rsp, b.condition(), "CompositionError").
				WithMessage(err.Error()).
				TargetComposite()
			return rsp, nil
		}

		if result == nil {
			// This means the builder decided to skip building/managing its resource,
			// so we just move on to the next builder without doing anything.
			log.Info("Skipping builder", "condition", b.condition())
			continue
		}

		// This updates a condition on the parent XR for each builder
		// So the XR status reflects whether that child resource is ready or not.
		if result.ready {
			response.ConditionTrue(rsp, b.condition(), "Available").
				TargetComposite()
		} else {
			response.ConditionFalse(rsp, b.condition(), "Unavailable").
				WithMessage(fmt.Sprintf("%s is not yet available", b.condition())).
				TargetComposite()
		}

		// Put the child resource into the final desired set
		// Later, Crossplane will use that desired set
		log.Info("Adding desired resource", "name", result.name)
		desired[result.name] = result.desired

		// If the builder returned any connection details, save it in the shared map
		// so we can publish it later in a connection secret.
		for k, v := range result.connectionDetails {
			connectionDetails[k] = v
		}
	}

	// If we've got any connection details, we create a Secret resource for them
	// and add it to the desired composed resources with a well-known name.
	// E.g., if the XR name is "my-db", the connection secret will be named "my-db-connection".
	if len(connectionDetails) > 0 {
		secretName := observedCompositeResource.Resource.GetName() + "-connection"
		secretResult, err := buildConnectionSecret(secretName, connectionDetails)
		if err != nil {
			runner.fatal(rsp, "cannot build connection secret", err)
			return rsp, nil
		}
		desired[secretResult.name] = secretResult.desired
	}

	// Finally, we write the full desired composed resources back into the function response.
	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		runner.fatal(rsp, "cannot set desired composed resources", err)
		return rsp, nil
	}

	return rsp, nil
}

// fatal is a helper method to set a failure condition on the response and log the error.
// It does the following:
//  1. Create a condition named FunctionSuccess
//  2. Set it to False
//  3. Use InternalError as the reason
//  4. Attach msg as the message
//  5. Publish that condition onto the composite resource and its claim
func (r *Runner[XR, D]) fatal(rsp *fnv1.RunFunctionResponse, msg string, err error) {
	// Add a FunctionSuccess=false condition with reason InternalError to the response.
	response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").
		WithMessage(msg).
		TargetCompositeAndClaim()
	response.Fatal(rsp, errors.Wrap(err, msg))
}

// buildResult is the output of a builder's process method.
// It contains the name of the resource, the desired composed resource, whether it's ready, and any connection details.
// Carries the output produced by a builder for a single reconciliation loop
type buildResult struct {
	name              resource.Name
	desired           *resource.DesiredComposed
	ready             bool
	connectionDetails map[string]string
}

// internalBuilder is the single adapter layer Runner works with.
// It hides resource-specific types and returns generic build results.
type internalBuilder[XR any, Defaults any] interface {
	condition() string
	process(ctx Context[XR, Defaults], observedComposedResources map[resource.Name]resource.ObservedComposed) (*buildResult, error)
}

// builderAdapter wraps a typed Builder and satisfies internalBuilder.
type builderAdapter[XR any, Defaults any, Observed runtime.Object, Desired runtime.Object] struct {
	builder Builder[XR, Defaults, Observed, Desired]
}

func (adapter *builderAdapter[XR, Defaults, Observed, Desired]) condition() string {
	return adapter.builder.Condition()
}

func (adapter *builderAdapter[XR, Defaults, Observed, Desired]) process(
	context Context[XR, Defaults],
	observedComposedResources map[resource.Name]resource.ObservedComposed,
) (*buildResult, error) {
	name := adapter.builder.ResourceName(context)

	if adapter.builder.Skip(context) {
		return nil, nil
	}

	// Get one decoded observed resource for this builder to use in its Desired, Ready, and Connection methods.
	// The runner receives observed child resources in a generic unstructured form,
	// but each builder wants a typed Go object.
	observedComposedResource, err := decodeObservedComposedResource[Observed](observedComposedResources, name)
	if err != nil {
		return nil, fmt.Errorf("decoding observed resource %q: %w", name, err)
	}

	// Ask the builder to build the desired resource based on the input context and the observed resource.
	desired, err := adapter.builder.Desired(context)
	if err != nil {
		return nil, fmt.Errorf("building desired resource %q: %w", name, err)
	}

	// If the builder returns a typed nil pointer, we treat it as nil and skip it.
	if isTypedNil(desired) {
		return nil, nil
	}

	// Convert the desired resource to a composed form.
	composedObj, err := composed.From(desired)
	if err != nil {
		return nil, fmt.Errorf("converting resource %q to composed form: %w", name, err)
	}

	ready := adapter.builder.Ready(context, observedComposedResource)
	readyState := resource.ReadyFalse
	if ready {
		readyState = resource.ReadyTrue
	}

	return &buildResult{
		name:              name,
		desired:           &resource.DesiredComposed{Resource: composedObj, Ready: readyState},
		ready:             ready,
		connectionDetails: adapter.builder.Connection(context, observedComposedResource),
	}, nil
}

// Convert the observed unstructured composed resource into a typed Go object a builder wants.
func decodeObservedComposedResource[T runtime.Object](
	observedComposedResources map[resource.Name]resource.ObservedComposed,
	name resource.Name,
) (T, error) {
	// Find the observed composed resource for this builder based on the name it specified.
	observedComposedResource, exists := observedComposedResources[name]
	if !exists {
		var zero T
		return zero, nil
	}

	// Create an empty typed object of type T that the builder wants
	output := newDecodeTarget[T]()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		observedComposedResource.Resource.UnstructuredContent(), output,
	); err != nil {
		var zero T
		return zero, err
	}

	return output, nil
}

// Create a connection secret resource with the given name and connection details,
// and convert it to a composed resource.
func buildConnectionSecret(name string, details map[string]string) (*buildResult, error) {
	// We create a normal Kubernetes Secret object with the connection details,
	// and then convert it to a composed resource so we can put it in the desired set.
	rawSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Type:       corev1.SecretTypeOpaque,
		Data:       make(map[string][]byte, len(details)),
	}
	for k, v := range details {
		rawSecret.Data[k] = []byte(v)
	}

	// Convert the Secret (a normal Kubernetes object) to a composed resource (a special format that Crossplane understands)
	// so we can put it in the desired set.
	secret, err := composed.From(rawSecret)
	if err != nil {
		return nil, err
	}

	// We return a buildResult with the name of the resource, the desired composed resource, and ready=true.
	return &buildResult{
		name:    resource.Name(name),
		desired: &resource.DesiredComposed{Resource: secret, Ready: resource.ReadyTrue},
	}, nil
}

// We need to create the right empty object for whatever XR actually is at runtime.
// We don't know the type of XR, whether it's *XDeployment, *XDatabase, or something else.
// This function simply create an empty value of type T that decoding can write into.
// If T is a pointer type, it returns a new pointer to the zero value of the element type, otherwise it returns the zero value of T.
// E.g. newDecodeTarget[*corev1.Secret]() returns a *corev1.Secret pointing to an empty Secret struct.
// Syntax explanation: this is a generic function which works with any type T, and returns a value of type T.
// Example of how to use generic functions in Go: https://go.dev/doc/tutorial/generics
//
//	func wrap[T any](v T) []T { return []T{v} }
//	What happens here:
//	T is set to string
//	So the function becomes (conceptually):
//	func wrap(v string) []string
func newDecodeTarget[T any]() T {
	// We need to handle both pointer and non-pointer types
	// .Elem() gives us the thing inside the pointer,
	// E.g., If the type is *int, t.Elem() is int, if the type is **corev1.Secret, t.Elem() is *corev1.Secret.
	//
	// reflect.TypeOf(...) returns the runtime type of the value
	// E.g.,
	// 	reflect.TypeOf("hello")     	// → string
	//  reflect.TypeOf((*string)(nil)) 	// → *string
	//  reflect.TypeOf(*corev1.Secret) 	// → **corev1.Secret
	//
	// (*T)(nil) A nil pointer of type *T
	// Used to get the reflect.Type of T without needing an actual value of type T.
	//
	t := reflect.TypeOf((*T)(nil)).Elem()
	if t.Kind() == reflect.Pointer {
		return reflect.New(t.Elem()).Interface().(T)
		// reflection trick explained:
		// A way to inspect and manipulate values at runtime, without knowing their type in advance
		// 		Normal Go value → 42 (you can use it directly)
		// 		reflect.Value → a box containing 42, with tools to inspect it
		// reflect.New(t.Elem()) ==> create a new empty value of the type inside the pointer, and return a pointer to it
		// 		E.g., if T is *corev1.Secret, t is *corev1.Secret, t.Elem() is corev1.Secret,
		// 		reflect.New(t.Elem()) creates a new corev1.Secret and returns &corev1.Secret{}.
		// .Interface() ==> convert reflection value to normal Go value
		// .(T) ==> treat it as type T
	}
	// If it's not a pointer, just return the zero value of T
	var zero T
	return zero
}

// isTypedNil reports whether v is nil or a typed nil pointer wrapped in an
// interface. In Go, a typed nil (e.g. (*corev1.Service)(nil)) stored in an
// interface is non-nil, so a plain == nil check is insufficient.
func isTypedNil(v any) bool {
	if v == nil {
		return true
	}
	val := reflect.ValueOf(v)
	return val.Kind() == reflect.Pointer && val.IsNil()
}
