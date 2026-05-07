package runner

import (
	"context"
	"testing"

	"github.com/crossplane/function-sdk-go/logging"
	fnv1 "github.com/crossplane/function-sdk-go/proto/v1"
	"github.com/crossplane/function-sdk-go/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type testObject struct {
	Name string
}

type testBuilder struct{}

func (testBuilder) Condition() string {
	return "ConfigMapReady"
}

func (testBuilder) ResourceName(Context[*corev1.ConfigMap, *corev1.ConfigMap]) resource.Name {
	return resource.Name("generated-config")
}

func (testBuilder) Skip(Context[*corev1.ConfigMap, *corev1.ConfigMap]) bool {
	return false
}

func (testBuilder) Desired(ctx Context[*corev1.ConfigMap, *corev1.ConfigMap]) (*corev1.ConfigMap, error) {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "generated-config",
		},
		Data: map[string]string{
			"xrName":    ctx.XR.GetName(),
			"inputName": ctx.Defaults.GetName(),
		},
	}, nil
}

func (testBuilder) Ready(Context[*corev1.ConfigMap, *corev1.ConfigMap], *corev1.ConfigMap) bool {
	return true
}

func (testBuilder) Connection(Context[*corev1.ConfigMap, *corev1.ConfigMap], *corev1.ConfigMap) map[string]string {
	return map[string]string{"endpoint": "example.internal"}
}

func TestNewTypedPointer(t *testing.T) {
	got := newDecodeTarget[*testObject]()
	if got == nil {
		t.Fatal("expected non-nil pointer")
	}
	if got.Name != "" {
		t.Fatalf("expected zero-value struct, got %q", got.Name)
	}
}

func TestNewTypedValue(t *testing.T) {
	got := newDecodeTarget[int]()
	if got != 0 {
		t.Fatalf("expected zero value, got %d", got)
	}
}

func TestIsTypedNil(t *testing.T) {
	var secret *corev1.Secret
	if !isTypedNil(secret) {
		t.Fatal("expected typed nil pointer to be treated as nil")
	}
	if isTypedNil(&corev1.Secret{}) {
		t.Fatal("expected non-nil pointer to not be treated as nil")
	}
	if !isTypedNil(nil) {
		t.Fatal("expected nil interface to be treated as nil")
	}
}

func TestDecodeObservedMissing(t *testing.T) {
	got, err := decodeObservedComposedResource[*corev1.Secret](map[resource.Name]resource.ObservedComposed{}, resource.Name("missing"))
	if err != nil {
		t.Fatalf("decodeObserved returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing resource, got %#v", got)
	}
}

func TestBuildConnectionSecret(t *testing.T) {
	result, err := buildConnectionSecret("example-connection", map[string]string{
		"username": "admin",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("buildConnectionSecret returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected build result")
	}
	if result.name != resource.Name("example-connection") {
		t.Fatalf("unexpected resource name: %q", result.name)
	}
	if result.desired == nil {
		t.Fatal("expected desired composed resource")
	}
	if result.desired.Ready != resource.ReadyTrue {
		t.Fatalf("expected ready state %q, got %q", resource.ReadyTrue, result.desired.Ready)
	}

	var secret corev1.Secret
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		result.desired.Resource.UnstructuredContent(),
		&secret,
	); err != nil {
		t.Fatalf("failed to decode desired secret: %v", err)
	}
	if string(secret.Data["username"]) != "admin" {
		t.Fatalf("unexpected username data: %q", string(secret.Data["username"]))
	}
	if string(secret.Data["password"]) != "secret" {
		t.Fatalf("unexpected password data: %q", string(secret.Data["password"]))
	}
	if secret.Type != corev1.SecretTypeOpaque {
		t.Fatalf("unexpected secret type: %q", secret.Type)
	}
}

func TestRunnerRunBuildsDesiredResources(t *testing.T) {
	req := &fnv1.RunFunctionRequest{
		Observed: &fnv1.State{
			Composite: &fnv1.Resource{
				Resource: resource.MustStructObject(&corev1.ConfigMap{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
					ObjectMeta: metav1.ObjectMeta{Name: "xr-sample"},
				}),
			},
			Resources: map[string]*fnv1.Resource{},
		},
		Desired: &fnv1.State{Resources: map[string]*fnv1.Resource{}},
		Input: resource.MustStructObject(&corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Name: "defaults-sample"},
		}),
	}

	r := New[*corev1.ConfigMap, *corev1.ConfigMap](req, logging.NewNopLogger())
	Register(r, testBuilder{})

	rsp, err := r.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if rsp == nil {
		t.Fatal("expected non-nil response")
	}

	resources := rsp.GetDesired().GetResources()
	if len(resources) != 2 {
		t.Fatalf("expected 2 desired resources, got %d", len(resources))
	}

	generated := resources["generated-config"]
	if generated == nil {
		t.Fatal("expected generated-config desired resource")
	}
	if generated.GetReady() != fnv1.Ready_READY_TRUE {
		t.Fatalf("expected generated-config ready true, got %v", generated.GetReady())
	}

	var config corev1.ConfigMap
	if err := resource.AsObject(generated.GetResource(), &config); err != nil {
		t.Fatalf("failed to decode desired configmap: %v", err)
	}
	if config.Data["xrName"] != "xr-sample" {
		t.Fatalf("unexpected xrName: %q", config.Data["xrName"])
	}
	if config.Data["inputName"] != "defaults-sample" {
		t.Fatalf("unexpected inputName: %q", config.Data["inputName"])
	}

	secretResource := resources["xr-sample-connection"]
	if secretResource == nil {
		t.Fatal("expected connection secret resource")
	}
	if secretResource.GetReady() != fnv1.Ready_READY_TRUE {
		t.Fatalf("expected connection secret ready true, got %v", secretResource.GetReady())
	}

	var secret corev1.Secret
	if err := resource.AsObject(secretResource.GetResource(), &secret); err != nil {
		t.Fatalf("failed to decode desired secret: %v", err)
	}
	if string(secret.Data["endpoint"]) != "example.internal" {
		t.Fatalf("unexpected endpoint connection detail: %q", string(secret.Data["endpoint"]))
	}
}
