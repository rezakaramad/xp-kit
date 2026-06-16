// Package xsimple is a minimal test fixture for the generator package.
// It defines a simple composite resource struct annotated with kubebuilder markers
// to verify that ExtractOpenAPISchema and BuildCompositeResourceDefinition work
// end-to-end against a real Go package.
package xsimple

// XSimple is a minimal composite resource used for testing the XRD generator.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type XSimple struct {
	Spec   XSimpleSpec   `json:"spec"`
	Status XSimpleStatus `json:"status,omitempty"`
}

// XSimpleSpec defines the desired state of XSimple.
type XSimpleSpec struct {
	// Name is the logical name of the resource.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Name string `json:"name"`

	// Replicas is the desired number of replicas.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// Region is the target cloud region.
	// +kubebuilder:validation:Enum=eu-west-1;eu-central-1;us-east-1
	Region string `json:"region,omitempty"`
}

// XSimpleStatus defines the observed state of XSimple.
type XSimpleStatus struct {
	// Ready indicates whether the resource is fully reconciled.
	Ready bool `json:"ready,omitempty"`
}
