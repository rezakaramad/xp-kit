// Package v1beta1 contains the input type for the xtenant-render Function.
// +kubebuilder:object:generate=true
// +groupName=platform.rezakara.demo
// +versionName=v1beta1
package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Input is the configuration passed to this Function from the Composition
// pipeline step. It configures the tenant bindings rendered into the GitOps
// and baseline ArgoCD Applications.
//
// +kubebuilder:object:root=true
// +kubebuilder:storageversion
// +kubebuilder:resource:categories=crossplane
type Input struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Github configures how rendered manifests are written to the export repository.
	// +optional
	Github GithubInput `json:"github,omitempty"`

	// Azure configures Entra-specific rendering behavior.
	// +optional
	Azure AzureInput `json:"azure,omitempty"`

	// Tenant contains the binding assignments rendered into the GitOps chart.
	Tenant TenantInput `json:"tenant"`

	// NextInsight configures optional Next-Insight metadata enrichment.
	// +optional
	NextInsight NextInsightInput `json:"nextInsight,omitempty"`
}

// GithubInput configures the Crossplane RepositoryFile resource written by this function.
type GithubInput struct {
	// ProviderConfigName references the GitHub provider config used for RepositoryFile resources.
	// +optional
	ProviderConfigName string `json:"providerConfigName,omitempty"`

	// CommitAuthor is the git author name used for rendered commits.
	// +optional
	CommitAuthor string `json:"commitAuthor,omitempty"`

	// CommitEmail is the git author email used for rendered commits.
	// +optional
	CommitEmail string `json:"commitEmail,omitempty"`
}

// WithDefaults returns a copy of g with any unset fields filled in with
// their default values.
func (g GithubInput) WithDefaults() GithubInput {
	if g.ProviderConfigName == "" {
		g.ProviderConfigName = "github-rezakaramad"
	}
	if g.CommitAuthor == "" {
		g.CommitAuthor = "Crossplane"
	}
	if g.CommitEmail == "" {
		g.CommitEmail = "crossplane@rezakara.demo"
	}
	return g
}

// AzureInput configures how Entra principals are provisioned.
type AzureInput struct {
	// PrincipalType selects whether the function creates Entra groups or users.
	// +kubebuilder:validation:Enum=group;user
	// +optional
	PrincipalType string `json:"principalType,omitempty"`

	// UserPrincipalDomain is used when principalType=user to set the UPN suffix.
	// +optional
	UserPrincipalDomain string `json:"userPrincipalDomain,omitempty"`
}

// TenantInput configures tenant-specific bindings rendered by the function.
type TenantInput struct {
	// Bindings associates a role with a cluster/environment pair.
	// +kubebuilder:validation:MinItems=1
	Bindings []BindingInput `json:"bindings"`
}

// BindingInput identifies a single tenant role-cluster binding.
type BindingInput struct {
	// Name is the logical role name for the binding (e.g. admin, viewer).
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Cluster is the ArgoCD destination cluster name.
	// +kubebuilder:validation:MinLength=1
	Cluster string `json:"cluster"`

	// EnvironmentPrefix is the short environment label used in resource naming
	// (e.g. dev, test, prod).
	// +kubebuilder:validation:MinLength=1
	EnvironmentPrefix string `json:"environmentPrefix"`
}

// NextInsightInput configures the optional Next-Insight metadata enrichment.
type NextInsightInput struct {
	// LabelPrefix is the Kubernetes label key prefix applied to all labels
	// produced from Next-Insight metadata (e.g. "nextinsight.rezakara.demo/").
	// +optional
	LabelPrefix string `json:"labelPrefix,omitempty"`
}
