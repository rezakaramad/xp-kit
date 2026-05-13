package generator

import (
	"encoding/json"

	apiextensionsv2 "github.com/crossplane/crossplane/v2/apis/apiextensions/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// MarshalXRDToYAML renders an apply-ready YAML CompositeResourceDefinition.
//
// The Status field is intentionally omitted from the output.
// Because apply-ready manifests should contain desired state, not server-populated live state.

// By convention in Go, Marshal... means “serialize this value into another format”.
// Here the target format is YAML.
// The function expects a pointer to a Crossplane CompositeResourceDefinition object type
// The function returns the serialized YAML as raw bytes, or an error if the marshalling process fails.
func MarshalXRDToYAML(xrd *apiextensionsv2.CompositeResourceDefinition) ([]byte, error) {
	// The full 'xrd' object has 4 top-level parts: TypeMeta, ObjectMeta, Spec, and Status.
	// Drop the status field from the output manifest since it is not part of
	// the desired state and is managed by Crossplane.
	manifest := struct {
		APIVersion string                                          `json:"apiVersion"`
		Kind       string                                          `json:"kind"`
		Metadata   metav1.ObjectMeta                               `json:"metadata"`
		Spec       apiextensionsv2.CompositeResourceDefinitionSpec `json:"spec"`
	}{
		APIVersion: xrd.TypeMeta.APIVersion,
		Kind:       xrd.TypeMeta.Kind,
		Metadata:   xrd.ObjectMeta,
		Spec:       xrd.Spec,
	}

	// json.Marshal produces bytes equivalent to the JSON representation of the manifest struct.
	jsonBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	// The yaml.JSONToYAML function converts JSON bytes to YAML format.
	return yaml.JSONToYAML(jsonBytes)
}
