package generator

import (
	"encoding/json"
	"strings"
	"testing"

	apiextensionsv2 "github.com/crossplane/crossplane/v2/apis/apiextensions/v2"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const testPackagePath = "github.com/rezakaramad/crossplane-toolkit/modules/generator/testdata/xsimple"

// TestExtractTypeInfo_Fields verifies that the schema extractor finds the expected
// fields and kubebuilder marker constraints on the XSimple test fixture.
func TestExtractTypeInfo_Fields(t *testing.T) {
	info, err := ExtractTypeInfo(testPackagePath, "test.example.org", "XSimple", "v1alpha1")
	if err != nil {
		t.Fatalf("ExtractTypeInfo: %v", err)
	}
	schema := info.Schema

	spec, ok := schema.Properties["spec"]
	if !ok {
		t.Fatal("expected 'spec' property in schema")
	}

	name, ok := spec.Properties["name"]
	if !ok {
		t.Fatal("expected 'name' property in spec")
	}
	if name.MinLength == nil || *name.MinLength != 1 {
		t.Errorf("expected MinLength=1 on 'name', got %v", name.MinLength)
	}
	if name.MaxLength == nil || *name.MaxLength != 63 {
		t.Errorf("expected MaxLength=63 on 'name', got %v", name.MaxLength)
	}

	replicas, ok := spec.Properties["replicas"]
	if !ok {
		t.Fatal("expected 'replicas' property in spec")
	}
	if replicas.Minimum == nil || *replicas.Minimum != 1 {
		t.Errorf("expected Minimum=1 on 'replicas', got %v", replicas.Minimum)
	}
	if replicas.Maximum == nil || *replicas.Maximum != 10 {
		t.Errorf("expected Maximum=10 on 'replicas', got %v", replicas.Maximum)
	}

	region, ok := spec.Properties["region"]
	if !ok {
		t.Fatal("expected 'region' property in spec")
	}
	if len(region.Enum) != 3 {
		t.Errorf("expected 3 enum values on 'region', got %d", len(region.Enum))
	}
}

// TestBuildCompositeResourceDefinition_Structure verifies the shape of an XRD built
// from the XSimple test fixture.
func TestBuildCompositeResourceDefinition_Structure(t *testing.T) {
	xrd, err := BuildCompositeResourceDefinition(ResourceMeta{
		PackagePath: testPackagePath,
		TypeName:    "XSimple",
		Group:       "test.example.org",
		Version:     "v1alpha1",
	})
	if err != nil {
		t.Fatalf("BuildCompositeResourceDefinition: %v", err)
	}

	if xrd.TypeMeta.APIVersion != "apiextensions.crossplane.io/v2" {
		t.Errorf("unexpected APIVersion: %q", xrd.TypeMeta.APIVersion)
	}
	if xrd.TypeMeta.Kind != "CompositeResourceDefinition" {
		t.Errorf("unexpected Kind: %q", xrd.TypeMeta.Kind)
	}
	if xrd.ObjectMeta.Name != "xsimples.test.example.org" {
		t.Errorf("unexpected name: %q", xrd.ObjectMeta.Name)
	}
	if xrd.Spec.Group != "test.example.org" {
		t.Errorf("unexpected group: %q", xrd.Spec.Group)
	}
	if xrd.Spec.Names.Kind != "XSimple" {
		t.Errorf("unexpected kind: %q", xrd.Spec.Names.Kind)
	}
	if xrd.Spec.Names.Plural != "xsimples" {
		t.Errorf("unexpected plural: %q", xrd.Spec.Names.Plural)
	}
	if xrd.Spec.Scope != apiextensionsv2.CompositeResourceScopeNamespaced {
		t.Errorf("unexpected scope: %q", xrd.Spec.Scope)
	}
	if len(xrd.Spec.Versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(xrd.Spec.Versions))
	}
	v := xrd.Spec.Versions[0]
	if v.Name != "v1alpha1" {
		t.Errorf("unexpected version name: %q", v.Name)
	}
	if !v.Served {
		t.Error("expected Served=true")
	}
	if !v.Referenceable {
		t.Error("expected Referenceable=true")
	}
}

// TestBuildCompositeResourceDefinition_DefaultVersion verifies that Version defaults
// to "v1alpha1" when not supplied.
func TestBuildCompositeResourceDefinition_DefaultVersion(t *testing.T) {
	xrd, err := BuildCompositeResourceDefinition(ResourceMeta{
		PackagePath: testPackagePath,
		TypeName:    "XSimple",
		Group:       "test.example.org",
	})
	if err != nil {
		t.Fatalf("BuildCompositeResourceDefinition: %v", err)
	}

	if len(xrd.Spec.Versions) == 0 {
		t.Fatal("expected at least one version")
	}
	if xrd.Spec.Versions[0].Name != "v1alpha1" {
		t.Errorf("expected default version v1alpha1, got %q", xrd.Spec.Versions[0].Name)
	}
}

// TestMarshalXRDToYAML verifies that MarshalXRDToYAML produces valid YAML
// containing the expected top-level keys and omits the status field.
func TestMarshalXRDToYAML(t *testing.T) {
	rawSchema, err := json.Marshal(apiextv1.JSONSchemaProps{Type: "object"})
	if err != nil {
		t.Fatalf("marshalling test schema: %v", err)
	}

	xrd := &apiextensionsv2.CompositeResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.crossplane.io/v2",
			Kind:       "CompositeResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "xsimples.test.example.org",
		},
		Spec: apiextensionsv2.CompositeResourceDefinitionSpec{
			Group: "test.example.org",
			Names: apiextv1.CustomResourceDefinitionNames{
				Kind:   "XSimple",
				Plural: "xsimples",
			},
			Versions: []apiextensionsv2.CompositeResourceDefinitionVersion{
				{
					Name:          "v1alpha1",
					Served:        true,
					Referenceable: true,
					Schema: &apiextensionsv2.CompositeResourceValidation{
						OpenAPIV3Schema: runtime.RawExtension{Raw: rawSchema},
					},
				},
			},
		},
	}

	out, err := MarshalXRDToYAML(xrd)
	if err != nil {
		t.Fatalf("MarshalXRDToYAML: %v", err)
	}

	yaml := string(out)
	for _, want := range []string{"apiVersion:", "kind:", "metadata:", "spec:"} {
		if !strings.Contains(yaml, want) {
			t.Errorf("expected YAML to contain %q", want)
		}
	}
	if strings.Contains(yaml, "status:") {
		t.Error("YAML output must not contain 'status:' field")
	}
}

// TestBuildCompositeResourceDefinition_ValidationErrors verifies that missing
// required fields are caught early with a clear error message.
func TestBuildCompositeResourceDefinition_ValidationErrors(t *testing.T) {
	cases := []struct {
		name    string
		input   ResourceMeta
		wantErr string
	}{
		{
			name:    "empty PackagePath",
			input:   ResourceMeta{TypeName: "XSimple", Group: "test.example.org"},
			wantErr: "PackagePath is required",
		},
		{
			name:    "empty TypeName",
			input:   ResourceMeta{PackagePath: testPackagePath, Group: "test.example.org"},
			wantErr: "TypeName is required",
		},
		{
			name:    "empty Group",
			input:   ResourceMeta{PackagePath: testPackagePath, TypeName: "XSimple"},
			wantErr: "Group is required",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildCompositeResourceDefinition(tc.input)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

// TestBuildCompositeResourceDefinition_ExplicitPlural verifies that an explicit
// Plural overrides the default lowercase-kind+s derivation.
func TestBuildCompositeResourceDefinition_ExplicitPlural(t *testing.T) {
	xrd, err := BuildCompositeResourceDefinition(ResourceMeta{
		PackagePath: testPackagePath,
		TypeName:    "XSimple",
		Group:       "test.example.org",
		Plural:      "xsimpleresources",
	})
	if err != nil {
		t.Fatalf("BuildCompositeResourceDefinition: %v", err)
	}

	if xrd.Spec.Names.Plural != "xsimpleresources" {
		t.Errorf("expected plural %q, got %q", "xsimpleresources", xrd.Spec.Names.Plural)
	}
	if xrd.ObjectMeta.Name != "xsimpleresources.test.example.org" {
		t.Errorf("expected name %q, got %q", "xsimpleresources.test.example.org", xrd.ObjectMeta.Name)
	}
}

// TestModuleContainsPackage verifies that moduleContainsPackage never matches
// a module path that is merely a string prefix of an unrelated module path.
func TestModuleContainsPackage(t *testing.T) {
	cases := []struct {
		modulePath  string
		packagePath string
		want        bool
	}{
		{"github.com/acme/app", "github.com/acme/app", true},
		{"github.com/acme/app", "github.com/acme/app/pkg/foo", true},
		{"github.com/acme/app", "github.com/acme/application", false},
		{"github.com/acme/app", "github.com/acme/application/pkg", false},
		{"github.com/acme/app", "github.com/other/app", false},
	}

	for _, tc := range cases {
		got := moduleContainsPackage(tc.modulePath, tc.packagePath)
		if got != tc.want {
			t.Errorf("moduleContainsPackage(%q, %q) = %v, want %v",
				tc.modulePath, tc.packagePath, got, tc.want)
		}
	}
}
