// Package argocd is a minimal stand-in for github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1.
// It provides just enough of the Application schema to construct desired resources in a Crossplane
// composition function and round-trip through runtime.DefaultUnstructuredConverter.
//
// To switch to the official package: replace the import paths in internal/baseline.go and
// internal/gitops.go with "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1", then
// delete this package. The argo-cd module currently has no types-only sub-module — importing it
// pulls in the full server stack as transitive dependencies.
package argocd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ApplicationKind is the Kubernetes Kind name for ArgoCD Application resources.
const ApplicationKind = "Application"

//nolint:gochecknoglobals // SchemeBuilder and AddToScheme are required globals for Kubernetes scheme registration
var (
	// GroupVersion is the group/version for ArgoCD application resources.
	GroupVersion = schema.GroupVersion{Group: "argoproj.io", Version: "v1alpha1"}

	// SchemeBuilder registers the ArgoCD types with a Kubernetes runtime scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(func(s *runtime.Scheme) error {
		s.AddKnownTypes(GroupVersion, &Application{}, &ApplicationList{})
		metav1.AddToGroupVersion(s, GroupVersion)
		return nil
	})

	// AddToScheme adds the ArgoCD types to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// Application is a minimal representation of an ArgoCD Application resource.
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ApplicationSpec `json:"spec"`
}

// ApplicationList contains a list of Applications.
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

// ApplicationSpec defines the desired state of an ArgoCD Application.
type ApplicationSpec struct {
	Project     string                 `json:"project"`
	Source      ApplicationSource      `json:"source"`
	Destination ApplicationDestination `json:"destination"`
	SyncPolicy  *SyncPolicy            `json:"syncPolicy,omitempty"`
}

// ApplicationSource describes the source of the application's manifests.
type ApplicationSource struct {
	RepoURL        string                 `json:"repoURL"`
	TargetRevision string                 `json:"targetRevision,omitempty"`
	Path           string                 `json:"path,omitempty"`
	Helm           *ApplicationSourceHelm `json:"helm,omitempty"`
}

// ApplicationSourceHelm holds Helm-specific source configuration.
type ApplicationSourceHelm struct {
	Values string `json:"values,omitempty"`
}

// ApplicationDestination describes the deployment target cluster and namespace.
type ApplicationDestination struct {
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// SyncPolicy controls how ArgoCD syncs the application.
type SyncPolicy struct {
	Automated *SyncPolicyAutomated `json:"automated,omitempty"`
}

// SyncPolicyAutomated enables automated sync with optional pruning and self-healing.
type SyncPolicyAutomated struct {
	Prune    bool `json:"prune,omitempty"`
	SelfHeal bool `json:"selfHeal,omitempty"`
}
