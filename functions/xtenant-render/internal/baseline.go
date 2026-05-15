// Package render provides helpers for building composed Crossplane resources.
package render

import (
	"fmt"
	"sort"

	argocdtypes "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-render/argocd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/function-sdk-go/resource/composed"
)

// BuildBaselineApplications creates one ArgoCD Application per unique destination cluster.
func BuildBaselineApplications(
	t TenantSpec,
	destinationClusters []Cluster,
	repo, branch, basePath string,
) ([]*composed.Unstructured, error) {
	if len(destinationClusters) == 0 {
		return nil, nil
	}

	// Ensure deterministic ordering (important for stable Git diffs).
	sorted := append([]Cluster(nil), destinationClusters...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Prefix < sorted[j].Prefix
	})

	var apps []*composed.Unstructured

	for _, c := range sorted {
		name := fmt.Sprintf("baseline-%s-%s", t.GetName(), c.Prefix)

		values := map[string]any{
			"tenant": map[string]any{
				metadataNameKey: t.GetName(),
				"dnsName":       t.Spec.DNSName,
			},
			"environmentPrefix": c.Prefix,
		}

		valuesYaml, err := yaml.Marshal(values)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal baseline values: %w", err)
		}

		app := &argocdtypes.Application{
			TypeMeta: metav1.TypeMeta{
				APIVersion: argocdtypes.GroupVersion.String(),
				Kind:       argocdtypes.ApplicationKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "argocd",
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":  managedByCrossplane,
					"platform.rezakara.demo/tenant": t.GetName(),
					"platform.rezakara.demo/prefix": c.Prefix,
				},
			},
			Spec: argocdtypes.ApplicationSpec{
				Project: "default",
				Source: argocdtypes.ApplicationSource{
					RepoURL:        repo,
					TargetRevision: branch,
					Path:           basePath,
					Helm: &argocdtypes.ApplicationSourceHelm{
						Values: string(valuesYaml),
					},
				},
				Destination: argocdtypes.ApplicationDestination{
					Name:      c.Name,
					Namespace: t.GetName(),
				},
			},
		}

		if t.Spec.ArgoCD.SyncPolicy.AutomatedSync {
			app.Spec.SyncPolicy = &argocdtypes.SyncPolicy{
				Automated: &argocdtypes.SyncPolicyAutomated{
					Prune:    t.Spec.ArgoCD.SyncPolicy.Prune,
					SelfHeal: t.Spec.ArgoCD.SyncPolicy.SelfHeal,
				},
			}
		}

		apps = append(apps, toComposed(app))
	}

	return apps, nil
}
