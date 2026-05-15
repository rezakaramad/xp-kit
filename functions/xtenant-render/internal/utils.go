package render

import (
	"bytes"
	"fmt"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/function-sdk-go/resource/composed"
)

// BundleYAML serializes one or more rendered Kubernetes resources into a
// single multi-document YAML string separated by "---" delimiters.
func BundleYAML(resources ...*composed.Unstructured) (string, error) {
	var buf bytes.Buffer
	first := true

	for _, resource := range resources {
		if resource == nil {
			continue
		}

		b, err := yaml.Marshal(resource.Object)
		if err != nil {
			return "", fmt.Errorf("cannot marshal resource %s/%s: %w",
				resource.GetKind(),
				resource.GetName(),
				err,
			)
		}

		if !first {
			buf.WriteString("---\n")
		}
		first = false

		buf.Write(b)
		if !bytes.HasSuffix(b, []byte("\n")) {
			buf.WriteString("\n")
		}
	}

	return buf.String(), nil
}

// generateAppRoleUUID produces a deterministic UUID for an ArgoCD app role
// instance scoped to (tenant, role, environment). Using SHA-1 over DNS
// namespace ensures the same input always produces the same UUID.
func generateAppRoleUUID(tenant, role, env string) string {
	seed := fmt.Sprintf("%s-%s-%s", tenant, role, env)
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(seed)).String()
}

// toComposed converts any typed Kubernetes resource (with json-tagged fields) into a
// *composed.Unstructured suitable for use in Crossplane composition function responses.
// Panics if conversion fails, which can only happen for types that cannot be JSON-marshalled
// (e.g. cyclic references or channel fields). Our generated types are all well-formed.
func toComposed(obj any) *composed.Unstructured {
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		panic(fmt.Sprintf("BUG: cannot convert typed resource to unstructured: %v", err))
	}
	return &composed.Unstructured{Unstructured: unstructured.Unstructured{Object: m}}
}
