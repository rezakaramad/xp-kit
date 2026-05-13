package generator

// This file takes the information from schema_extractor.go
// and builds a Kubernetes object that Crossplane expects.

import (
	"encoding/json"
	"fmt"
	"strings"

	apiextensionsv2 "github.com/crossplane/crossplane/v2/apis/apiextensions/v2"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Contains the fields needed to generate a CompositeResourceDefinition.
type ResourceMeta struct {
	PackagePath           string // Go package containing the type to be converted into an XRD
	TypeName              string // Name of the Go type to be converted into an XRD
	Group                 string // API group for the XRD
	Version               string // API version for the XRD (optional, defaults to "v1alpha1")
	Plural                string // Plural name for the XRD
	Scope                 string // XRD scope: "Namespaced" or "Cluster" (optional, defaults to "Namespaced")
	DefaultCompositionRef string // Name of the default Composition to use when none is specified on the XR
}

// It does the following things:
//  1. Validates the required input
//  2. Selects the XRD API version name, defaulting to "v1alpha1" if none is provided
//  3. Asks ExtractTypeInfo to get the OpenAPI schema and additionalPrinterColumns for the specified Go type in one pass
//  4. Takes only the 'spec' part of the schema (desired state) and wraps it in a top-level schema with 'spec' and 'status' properties.
func BuildCompositeResourceDefinition(resource ResourceMeta) (*apiextensionsv2.CompositeResourceDefinition, error) {
	if resource.PackagePath == "" {
		return nil, fmt.Errorf("PackagePath is required")
	}
	if resource.TypeName == "" {
		return nil, fmt.Errorf("TypeName is required")
	}
	if resource.Group == "" {
		return nil, fmt.Errorf("Group is required")
	}

	version := resource.Version
	if version == "" {
		version = "v1alpha1"
	}

	scope := apiextensionsv2.CompositeResourceScope(resource.Scope)
	if scope == "" {
		scope = apiextensionsv2.CompositeResourceScopeNamespaced
	}

	// Load the Go type once and extract both the OpenAPI schema and printer columns.
	typeInfo, err := ExtractTypeInfo(resource.PackagePath, resource.Group, resource.TypeName, version)
	if err != nil {
		return nil, fmt.Errorf("extracting type info: %w", err)
	}

	// Gets the 'spec' part of the schema, which represents the desired state of the resource.
	specSchema, ok := typeInfo.Schema.Properties["spec"]
	if !ok {
		return nil, fmt.Errorf("no 'spec' field found in schema for %q", resource.TypeName)
	}

	// Wrap the 'spec' schema in a top-level schema.
	// Copy root-level XValidations (CEL rules) and XPreserveUnknownFields from the
	// parsed root schema so markers placed on the XR type itself are preserved.
	wrappedSchema := apiextv1.JSONSchemaProps{
		Type:                   "object",
		Required:               []string{"spec"},
		XValidations:           typeInfo.Schema.XValidations,
		XPreserveUnknownFields: typeInfo.Schema.XPreserveUnknownFields,
		Properties: map[string]apiextv1.JSONSchemaProps{
			"spec": specSchema,
		},
	}

	// Include the 'status' schema if it exists.
	if statusSchema, ok := typeInfo.Schema.Properties["status"]; ok {
		wrappedSchema.Properties["status"] = statusSchema
	}

	// Marshal the wrapped schema into JSON bytes, which will be embedded in the XRD.
	rawSchema, err := json.Marshal(wrappedSchema)
	if err != nil {
		return nil, fmt.Errorf("marshalling schema: %w", err)
	}

	// Build the CompositeResourceDefinition object with the appropriate metadata and spec.
	kind := resource.TypeName
	plural := resource.Plural
	if plural == "" {
		plural = strings.ToLower(kind) + "s"
	}

	return &apiextensionsv2.CompositeResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.crossplane.io/v2",
			Kind:       "CompositeResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: plural + "." + resource.Group,
		},
		Spec: apiextensionsv2.CompositeResourceDefinitionSpec{
			Group: resource.Group,
			Scope: scope,
			Names: apiextv1.CustomResourceDefinitionNames{
				Kind:   kind,
				Plural: plural,
			},
			DefaultCompositionRef: func() *apiextensionsv2.CompositionReference {
				if resource.DefaultCompositionRef == "" {
					return nil
				}
				return &apiextensionsv2.CompositionReference{Name: resource.DefaultCompositionRef}
			}(),
			Versions: []apiextensionsv2.CompositeResourceDefinitionVersion{
				{
					Name:                     version,
					Served:                   true,
					Referenceable:            true,
					AdditionalPrinterColumns: typeInfo.PrinterColumns,
					Schema: &apiextensionsv2.CompositeResourceValidation{
						OpenAPIV3Schema: runtime.RawExtension{
							Raw: rawSchema,
						},
					},
				},
			},
		},
	}, nil
}
