// Package generator provides utilities to generate Crossplane XRDs from Go types.
package generator

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	apiextensionsv2 "github.com/crossplane/crossplane/v2/apis/apiextensions/v2"
)

// MarshalXRDToYAML renders an apply-ready YAML CompositeResourceDefinition.
// The Status field is intentionally omitted because apply-ready manifests should
// contain desired state, not server-populated live state.
func MarshalXRDToYAML(xrd *apiextensionsv2.CompositeResourceDefinition) ([]byte, error) {
	manifest := struct {
		APIVersion string                                          `json:"apiVersion"`
		Kind       string                                          `json:"kind"`
		Metadata   metav1.ObjectMeta                               `json:"metadata"`
		Spec       apiextensionsv2.CompositeResourceDefinitionSpec `json:"spec"`
	}{
		APIVersion: xrd.APIVersion,
		Kind:       xrd.Kind,
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
