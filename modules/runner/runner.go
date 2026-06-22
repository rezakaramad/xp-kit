package runner

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/crossplane/function-sdk-go/errors"
	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/resource"
	"github.com/crossplane/function-sdk-go/resource/composed"
	"github.com/crossplane/function-sdk-go/response"
)

// The runner creates a Kubernetes Secret when composers return connection details.
func init() { //nolint:gochecknoinits
	if err := corev1.AddToScheme(composed.Scheme); err != nil {
		panic(fmt.Sprintf("register corev1 types in composed scheme: %v", err))
	}
}

// Runner is used inside a Crossplane function's RunFunction method.
// Every time Crossplane calls your function’s RunFunction, you create a fresh Runner
type Runner[XR any, Input runtime.Object] struct {
	req       *fnv1.RunFunctionRequest
	log       logging.Logger
	composers []composerExecutor[XR, Input]
}

// New creates a Runner for the given request and logger.
func New[XR any, Input runtime.Object](
	req *fnv1.RunFunctionRequest,
	log logging.Logger,
) *Runner[XR, Input] {
	return &Runner[XR, Input]{
		req: req,
		log: log,
	}
}

// Register adds a typed Resource to the Runner.
func Register[
	XR any,
	Input runtime.Object,
	Observed runtime.Object,
	Desired runtime.Object,
](
	r *Runner[XR, Input],
	res Composer[XR, Input, Observed, Desired],
) {
	r.composers = append(
		r.composers,
		&composerAdapter[XR, Input, Observed, Desired]{
			composer: res,
		})
}

// ─── Run ─────────────────────────────────────────────────────────────────────
// Run executes the full function lifecycle and returns the populated response.
func (r *Runner[XR, Input]) Run(
	ctx context.Context,
) (*fnv1.RunFunctionResponse, error) {
	// Create a response with the same meta as the request and a default TTL.
	rsp := response.To(r.req, response.DefaultTTL)

	// ── 1. Decode observed XR ────────────────────────────────────────────────
	observedXR, err := request.GetObservedCompositeResource(r.req)
	if err != nil {
		r.fatal(rsp, "cannot get observed composite resource", err)
		return rsp, nil
	}

	// Decode the observed XR into a typed struct so builders can read from it in a type-safe way.
	// We use a helper function to allocate a zero value of the right type that is safe to write into, even when XR is a pointer type (e.g. *MyXR).
	xr := newDecodeTarget[XR]()
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		observedXR.Resource.UnstructuredContent(), xr,
	); err != nil {
		r.fatal(rsp, "cannot decode composite resource", err)
		return rsp, nil
	}

	// ── 2. Decode step input ─────────────────────────────────────────────────
	input := newDecodeTarget[Input]()
	if err := request.GetInput(r.req, input); err != nil {
		r.fatal(rsp, "cannot get function input", err)
		return rsp, nil
	}

	// ── 3. Collect observed composed resources ───────────────────────────────
	observedComposed, err := request.GetObservedComposedResources(r.req)
	if err != nil {
		r.fatal(rsp, "cannot get observed composed resources", err)
		return rsp, nil
	}

	// ── 4. Collect desired composed resources ────────────────────────────────
	desired, err := request.GetDesiredComposedResources(r.req)
	if err != nil {
		r.fatal(rsp, "cannot get desired composed resources", err)
		return rsp, nil
	}

	// ── 5. Build Context ─────────────────────────────────────────────────────
	fnCtx := Context[XR, Input]{
		Ctx:      ctx,
		XR:       xr,
		Input:    input,
		Observed: observedComposed,
		Log: r.log.WithValues(
			"xr-version", observedXR.Resource.GetAPIVersion(),
			"xr-kind", observedXR.Resource.GetKind(),
			"xr-name", observedXR.Resource.GetName(),
		),
	}

	// ── 6. Call each resource ───────────────────────────────────────────────
	connectionDetails := map[string]string{}

	for _, composer := range r.composers {
		result, err := composer.process(fnCtx)
		if err != nil {
			if condType := composer.conditionType(); condType != "" {
				response.ConditionFalse(rsp, condType, conditionReason(err)).
					WithMessage(err.Error()).
					TargetComposite()
			}
			return rsp, nil
		}

		if result == nil {
			fnCtx.Log.Info("Skipping resource", "condition", composer.conditionType())
			continue
		}

		condType := composer.conditionType()
		if condType != "" {
			if result.ready {
				response.ConditionTrue(rsp, condType, "Available").TargetComposite()
			} else {
				response.ConditionFalse(rsp, condType, "Unavailable").
					WithMessage(fmt.Sprintf("%s is not yet available", condType)).
					TargetComposite()
			}
		}

		fnCtx.Log.Info("Adding desired composer resource", "name", result.name)
		desired[result.name] = result.desired

		// Merge this composer's connection details into the shared map; all composers contribute to one Secret.
		for k, v := range result.connectionDetails {
			connectionDetails[k] = v
		}
	}

	// ── 7. Publish connection secret ─────────────────────────────────────────
	if len(connectionDetails) > 0 {
		secretResult, err := buildConnectionSecret(observedXR, connectionDetails)
		if err != nil {
			r.fatal(rsp, "cannot build connection secret", err)
			return rsp, nil
		}
		desired[secretResult.name] = secretResult.desired
	}

	if err := response.SetDesiredComposedResources(rsp, desired); err != nil {
		r.fatal(rsp, "cannot set desired composed resources", err)
		return rsp, nil
	}

	return rsp, nil
}

// ─── Internal helpers ────────────────────────────────────────────────────────

// fatal sets FunctionSuccess=False on the response and marks the pipeline as
// failed. This stops Crossplane from proceeding with subsequent pipeline steps.
func (r *Runner[XR, Input]) fatal(rsp *fnv1.RunFunctionResponse, msg string, err error) {
	response.ConditionFalse(rsp, "FunctionSuccess", "InternalError").
		WithMessage(msg).
		TargetCompositeAndClaim()
	response.Fatal(rsp, errors.Wrap(err, msg))
}

// buildConnectionSecret resolves the Secret name (from spec.writeConnectionSecretToRef.name,
// or "<xr-name>-connection" as fallback), then builds the Secret from the merged
// connection details and converts it to a composed resource.
func buildConnectionSecret(xr *resource.Composite, details map[string]string) (*buildResult, error) {
	name := xr.Resource.GetName() + "-connection"
	if spec, ok := xr.Resource.UnstructuredContent()["spec"].(map[string]interface{}); ok {
		if ref, ok := spec["writeConnectionSecretToRef"].(map[string]interface{}); ok {
			if n, ok := ref["name"].(string); ok && n != "" {
				name = n
			}
		}
	}
	rawSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Type:       corev1.SecretTypeOpaque,
		Data:       make(map[string][]byte, len(details)),
	}
	for k, v := range details {
		rawSecret.Data[k] = []byte(v)
	}

	secret, err := composed.From(rawSecret)
	if err != nil {
		return nil, err
	}

	return &buildResult{
		name:    resource.Name(name),
		desired: &resource.DesiredComposed{Resource: secret, Ready: resource.ReadyTrue},
	}, nil
}

// Returns an empty object of type T that is safe to write data into..
func newDecodeTarget[T any]() T {
	var zero T
	// Look at zero and see what kind of thing it is..
	v := reflect.ValueOf(&zero).Elem()
	//
	if v.Kind() == reflect.Pointer && v.IsNil() {
		// Return &MyXR{} instead of nil when T is a pointer type, so the caller can write into it.
		v.Set(reflect.New(v.Type().Elem()))
	}
	return zero
}

// isTypedNil reports true when v is a nil pointer wrapped in an interface.
// reflect.ValueOf((*T)(nil)) is not nil at the interface level, so a plain
// v == nil check misses this case.
func isTypedNil(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}

// ─── Internal adapter types ──────────────────────────────────────────────────

// buildResult carries the output of one resource for a single reconcile.
type buildResult struct {
	name              resource.Name
	desired           *resource.DesiredComposed
	ready             bool
	connectionDetails map[string]string
}

// composerExecutor is the type-erased interface the Runner works with.
// It hides the Observed/Desired type parameters of the concrete Composer.
type composerExecutor[XR any, Input runtime.Object] interface {
	conditionType() string
	process(ctx Context[XR, Input]) (*buildResult, error)
}

// composerAdapter wraps a typed Composer[XR,D,Obs,Des] and satisfies
// internalComposer[XR,D]. It is the only place where the four type parameters
// appear together; everything else in the runner sees only two.
type composerAdapter[XR any, Input runtime.Object, Observed runtime.Object, Desired runtime.Object] struct {
	composer Composer[XR, Input, Observed, Desired]
}

func (a *composerAdapter[XR, Input, Observed, Desired]) conditionType() string { //nolint:unused
	return a.composer.ConditionType()
}

func (a *composerAdapter[XR, Input, Observed, Desired]) process( //nolint:unused
	ctx Context[XR, Input],
) (*buildResult, error) {
	name := a.composer.ResourceName(ctx)

	obs, err := ObservedAs[Observed](ctx.Observed, name)
	if err != nil {
		return nil, fmt.Errorf("decoding observed resource %q: %w", name, err)
	}

	desired, err := a.composer.Compose(ctx)
	if err != nil {
		return nil, fmt.Errorf("composing desired resource %q: %w", name, err)
	}

	if isTypedNil(desired) {
		return nil, nil
	}

	composedObj, err := composed.From(desired)
	if err != nil {
		return nil, fmt.Errorf("converting resource %q to composed form: %w", name, err)
	}

	ready := a.composer.IsReady(ctx, obs)
	readyState := resource.ReadyFalse
	if ready {
		readyState = resource.ReadyTrue
	}

	return &buildResult{
		name:              name,
		desired:           &resource.DesiredComposed{Resource: composedObj, Ready: readyState},
		ready:             ready,
		connectionDetails: a.composer.ConnectionDetails(ctx, obs),
	}, nil
}
