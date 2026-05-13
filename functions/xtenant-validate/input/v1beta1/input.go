// Package v1beta1 contains the input type for the xtenant-validate Function.
// +kubebuilder:object:generate=true
// +groupName=platform.rezakara.demo
// +versionName=v1beta1
package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Input is the configuration passed to this Function from the Composition
// pipeline step. It configures the platform validation settings used by the
// tenant validator.
//
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type Input struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// DNS configures DNS validation behavior.
	DNS DNSInput `json:"dns"`

	// Clusters lists the workload clusters and prefixes the platform exposes.
	// +kubebuilder:validation:MinItems=1
	Clusters []ClusterInput `json:"clusters"`
}

// DNSInput configures DNS validation behavior.
type DNSInput struct {
	// BaseDomain is the DNS suffix used when validating tenant hostnames.
	// +kubebuilder:validation:MinLength=1
	BaseDomain string `json:"baseDomain"`

	// Provider selects which DNS backend to use for availability checks.
	// +kubebuilder:validation:Enum=powerdns;clouddns
	// +kubebuilder:default=powerdns
	Provider string `json:"provider"`

	// APIURL is the base URL of the PowerDNS API.
	// Required when provider is "powerdns"; ignored otherwise.
	// +optional
	APIURL string `json:"apiURL,omitempty"`

	// GCPProject is the Google Cloud project that owns the Cloud DNS managed
	// zone. Required when provider is "clouddns"; ignored otherwise.
	// +optional
	GCPProject string `json:"gcpProject,omitempty"`

	// CredentialsSecretRef references a Kubernetes Secret that holds the API
	// key for the DNS provider. The function reads this secret on every
	// reconcile, so key rotation is picked up automatically without a pod
	// restart. Required when provider is "powerdns".
	// +optional
	CredentialsSecretRef *SecretKeyRef `json:"credentialsSecretRef,omitempty"`
}

// SecretKeyRef identifies a single key inside a Kubernetes Secret.
type SecretKeyRef struct {
	// Namespace of the Secret.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`

	// Name of the Secret.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key is the data key within the Secret whose value is the API key.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// ClusterInput identifies a workload cluster and its environment prefix.
type ClusterInput struct {
	// Name is the logical workload cluster name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Prefix is the DNS/environment prefix associated with the cluster.
	// +kubebuilder:validation:MinLength=1
	Prefix string `json:"prefix"`
}
