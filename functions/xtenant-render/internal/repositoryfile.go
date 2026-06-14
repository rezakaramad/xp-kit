package render

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	repov1alpha1 "github.com/crossplane-contrib/provider-upjet-github/apis/namespaced/repo/v1alpha1"
	commonv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	commonv2 "github.com/crossplane/crossplane-runtime/v2/apis/common/v2"
	"github.com/crossplane/function-sdk-go/resource/composed"
)

// RepositoryFileConfig provides all external settings required to build a RepositoryFile resource.
type RepositoryFileConfig struct {
	ProviderConfigName string
	Repository         string
	Branch             string
	BasePath           string
	CommitAuthor       string
	CommitEmail        string
}

// BuildRepositoryFile constructs a Crossplane RepositoryFile composed resource.
// The RepositoryFile tells the GitHub provider to write the rendered bundle YAML
// into the export repository so that ArgoCD can pick it up and deploy it.
func BuildRepositoryFile(t TenantSpec, content string, cfg RepositoryFileConfig) *composed.Unstructured {
	path := fmt.Sprintf("%s/%s/bundle.yaml",
		strings.TrimSuffix(cfg.BasePath, "/"),
		t.GetName(),
	)

	hash := sha256.Sum256([]byte(content))
	shortHash := hex.EncodeToString(hash[:])[:8]
	commitMsg := fmt.Sprintf("Render tenant %s manifests (%s)", t.GetName(), shortHash)
	overwrite := true

	rf := &repov1alpha1.RepositoryFile{
		TypeMeta: metav1.TypeMeta{
			APIVersion: repov1alpha1.CRDGroupVersion.String(),
			Kind:       repov1alpha1.RepositoryFile_Kind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-bundle", t.GetName()),
			Labels: commonLabels(t),
		},
		Spec: repov1alpha1.RepositoryFileSpec{
			ForProvider: repov1alpha1.RepositoryFileParameters{
				Repository:        &cfg.Repository,
				Branch:            &cfg.Branch,
				File:              &path,
				Content:           &content,
				CommitAuthor:      &cfg.CommitAuthor,
				CommitEmail:       &cfg.CommitEmail,
				CommitMessage:     &commitMsg,
				OverwriteOnCreate: &overwrite,
			},
			ManagedResourceSpec: commonv2.ManagedResourceSpec{
				ProviderConfigReference: &commonv1.ProviderConfigReference{
					Name: cfg.ProviderConfigName,
					Kind: "ClusterProviderConfig",
				},
			},
		},
	}

	return toComposed(rf)
}
