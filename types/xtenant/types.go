package xtenant

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Phase represents the lifecycle state of an XTenant.
//
// +kubebuilder:validation:Enum=Validating;PendingApproval;Provisioning;Ready;Failed
type Phase string

// Phase constants define the lifecycle phases of an XTenant.
const (
	PhaseValidating      Phase = "Validating"
	PhasePendingApproval Phase = "PendingApproval"
	PhaseProvisioning    Phase = "Provisioning"
	PhaseReady           Phase = "Ready"
	PhaseFailed          Phase = "Failed"
)

// XTenant is the strongly-typed representation of the Tenant Composite Resource.
//
// +kubebuilder:object:root=true
// +kubebuilder:validation:XValidation:rule="self.metadata.name.size() <= 20",message="Tenant name must be 20 characters or less"
// +kubebuilder:validation:XValidation:rule="self.metadata.name.matches('^[a-z0-9]+(-[a-z0-9]+)*$')",message="Tenant name must be lowercase alphanumeric with hyphens"
// +kubebuilder:validation:XValidation:rule="!self.metadata.name.matches('(^|-)(dev|test|stage|prod)(-|$)')",message="Tenant name must not include reserved environment segments (dev, test, stage, prod)"
// +kubebuilder:validation:XValidation:rule="oldSelf == null || self.spec.dnsName == oldSelf.spec.dnsName",message="spec.dnsName is immutable after creation"
// +kubebuilder:validation:XValidation:rule="oldSelf == null || !oldSelf.spec.approved || self.spec.approved",message="spec.approved cannot be set back to false once approved"
// +kubebuilder:printcolumn:name="Tenant",type="string",JSONPath=".metadata.name"
// +kubebuilder:printcolumn:name="DNS",type="string",JSONPath=".spec.dnsName"
// +kubebuilder:printcolumn:name="Team",type="string",JSONPath=".spec.teamID"
// +kubebuilder:printcolumn:name="Resources",type="integer",JSONPath=".status.rendered.resources"
// +kubebuilder:printcolumn:name="Approved",type="boolean",JSONPath=".spec.approved"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
type XTenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              XTenantSpec `json:"spec"`

	// +kubebuilder:pruning:PreserveUnknownFields
	Status XTenantStatus `json:"status,omitempty"`
}

// XTenantSpec defines the desired state of an XTenant.
type XTenantSpec struct { //nolint:revive // XTenant prefix is intentional for clarity across packages
	// crossplane is reserved for Crossplane-specific implementation details.
	//
	// +kubebuilder:pruning:PreserveUnknownFields
	Crossplane runtime.RawExtension `json:"crossplane,omitempty"`

	// dnsName is the base DNS label for the tenant. Immutable after creation.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	DNSName string `json:"dnsName"`

	// displayName is a human-readable name shown in UIs.
	//
	// +kubebuilder:validation:MaxLength=128
	DisplayName string `json:"displayName,omitempty"`

	// argocd contains ArgoCD-specific configuration.
	ArgoCD ArgoCDSpec `json:"argocd,omitempty"`

	// options contains optional metadata and cost allocation fields.
	Options OptionsSpec `json:"options,omitempty"`

	// approved gates provisioning. Must be set to true by a platform engineer
	// before function-tenant-renderer will create any resources.
	// Once set to true it cannot be reverted.
	//
	// +kubebuilder:default=false
	Approved bool `json:"approved,omitempty"`

	// teamID is the Next-Insight identifier for the Agile team that owns this
	// tenant. When set, the render function enriches namespace labels with
	// ownership metadata (Agile Release Train and Agile Team) fetched from
	// the Next-Insight API.
	//
	// +kubebuilder:validation:MaxLength=128
	// +optional
	TeamID string `json:"teamID,omitempty"`
}

// ArgoCDSpec contains ArgoCD-specific configuration for the tenant.
type ArgoCDSpec struct {
	SyncPolicy SyncPolicySpec `json:"syncPolicy,omitempty"`
}

// SyncPolicySpec configures ArgoCD automated sync behaviour.
type SyncPolicySpec struct {
	// +kubebuilder:default=true
	AutomatedSync bool `json:"automatedSync,omitempty"`

	// +kubebuilder:default=true
	Prune bool `json:"prune,omitempty"`

	// +kubebuilder:default=true
	SelfHeal bool `json:"selfHeal,omitempty"`
}

// OptionsSpec contains optional metadata and cost allocation fields.
type OptionsSpec struct {
	// +kubebuilder:validation:MaxLength=64
	CostCenter  string            `json:"costCenter,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// XTenantStatus defines the observed state of an XTenant.
type XTenantStatus struct { //nolint:revive // XTenant prefix is intentional for clarity across packages
	Phase              Phase              `json:"phase,omitempty"`
	Rendered           *RenderedStatus    `json:"rendered,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

// RenderedStatus summarises the resources exported to Git.
type RenderedStatus struct {
	Resources int    `json:"resources,omitempty"`
	Message   string `json:"message,omitempty"`
}
