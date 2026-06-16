package xdeployment

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// XDeployment is the strongly-typed representation of the Deployment Composite Resource.
//
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="App",type="string",JSONPath=".metadata.name"
// +kubebuilder:printcolumn:name="Image",type="string",JSONPath=".spec.image"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".spec.replicas"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
type XDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              XDeploymentSpec `json:"spec"`

	// +kubebuilder:pruning:PreserveUnknownFields
	Status XDeploymentStatus `json:"status,omitempty"`
}

// XDeploymentSpec defines the desired state of an XDeployment.
type XDeploymentSpec struct {
	// crossplane is reserved for Crossplane-specific implementation details.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	Crossplane runtime.RawExtension `json:"crossplane,omitempty"`

	// image is the container image to deploy, e.g. "nginx:1.25".
	//
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// replicas is the desired number of pod replicas.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	Replicas int32 `json:"replicas,omitempty"`

	// appID is the Next-Insight application identifier. When set, the function
	// enriches the Deployment labels with application metadata fetched from
	// the Next-Insight API.
	//
	// +optional
	AppID string `json:"appId,omitempty"`

	// namespace is the target namespace for the Deployment.
	//
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
}

// XDeploymentStatus reflects the observed state of an XDeployment.
type XDeploymentStatus struct {
	// phase is the current lifecycle phase of the deployment.
	Phase string `json:"phase,omitempty"`
}
