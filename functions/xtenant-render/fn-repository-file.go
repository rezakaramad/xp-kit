package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/crossplane/function-sdk-go/resource/composed"
)

// RepositoryFileConfig provides all external settings required to build a RepositoryFile resource.
type RepositoryFileConfig struct {
	Namespace          string
	ProviderConfigName string
	Repository         string
	Branch             string
	BasePath           string
	CommitAuthor       string
	CommitEmail        string
}

// buildRepositoryFile constructs a Crossplane RepositoryFile composed resource.
// The RepositoryFile tells the GitHub provider to write the rendered bundle YAML
// into the export repository so that ArgoCD can pick it up and deploy it.
func buildRepositoryFile(t TenantSpec, content string, cfg RepositoryFileConfig) *composed.Unstructured {
	path := fmt.Sprintf("%s/%s/bundle.yaml",
		strings.TrimSuffix(cfg.BasePath, "/"),
		t.GetName(),
	)

	hash := sha256.Sum256([]byte(content))
	shortHash := hex.EncodeToString(hash[:])[:8]

	u := &unstructured.Unstructured{}
	u.SetAPIVersion("repo.github.m.upbound.io/v1alpha1")
	u.SetKind("RepositoryFile")
	u.SetName(fmt.Sprintf("%s-bundle", t.GetName()))

	ns := cfg.Namespace
	if ns == "" {
		ns = defaultCrossplaneNamespace
	}
	u.SetNamespace(ns)
	u.SetLabels(commonLabels(t))

	u.Object["spec"] = map[string]any{
		"forProvider": map[string]any{
			"repository":        cfg.Repository,
			"branch":            cfg.Branch,
			"file":              path,
			"content":           content,
			"commitAuthor":      cfg.CommitAuthor,
			"commitEmail":       cfg.CommitEmail,
			"commitMessage":     fmt.Sprintf("Render tenant %s manifests (%s)", t.GetName(), shortHash),
			"overwriteOnCreate": true,
		},
		"providerConfigRef": map[string]any{
			metadataNameKey: cfg.ProviderConfigName,
			"kind":          "ClusterProviderConfig",
		},
	}

	return &composed.Unstructured{Unstructured: *u}
}
