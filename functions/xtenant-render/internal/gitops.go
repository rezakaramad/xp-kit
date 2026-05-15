package render

import (
	"fmt"
	"sort"

	argocdtypes "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-render/argocd"
	inputv1beta1 "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-render/input/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/crossplane/function-sdk-go/resource/composed"
)

func policiesForRole(role string) ([]map[string]any, error) {
	switch role {
	case "admin":
		return []map[string]any{
			{
				"resource": "applications",
				"actions":  []string{"get", "update", "delete", "sync", "action/apps/Deployment/pause", "action/apps/Deployment/resume", "action/apps/Deployment/restart", "action/batch/CronJob/create-job"},
			},
			{
				"resource": "logs",
				"actions":  []string{"get"},
			},
		}, nil
	case "viewer":
		return []map[string]any{
			{
				"resource": "applications",
				"actions":  []string{"get"},
			},
			{
				"resource": "logs",
				"actions":  []string{"get"},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported role %q", role)
	}
}

func renderedRoles(bindings []ResolvedBinding) ([]map[string]any, error) {
	seen := map[string]struct{}{}
	roles := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if _, ok := seen[binding.Role]; ok {
			continue
		}
		seen[binding.Role] = struct{}{}
		roles = append(roles, binding.Role)
	}
	sort.Strings(roles)

	out := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		policies, err := policiesForRole(role)
		if err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			metadataNameKey: role,
			"policies":      policies,
		})
	}
	return out, nil
}

// BuildGitopsApplication creates the ArgoCD gitops Application for a tenant.
func BuildGitopsApplication(
	t TenantSpec,
	bindings []ResolvedBinding,
	azure inputv1beta1.AzureInput,
	repo, branch, basePath string,
) (*composed.Unstructured, error) {
	name := fmt.Sprintf("gitops-%s", t.GetName())

	roles, err := renderedRoles(bindings)
	if err != nil {
		return nil, fmt.Errorf("cannot render tenant roles: %w", err)
	}

	renderedBindings := make([]map[string]any, 0, len(bindings))
	for _, binding := range bindings {
		renderedBindings = append(renderedBindings, map[string]any{
			"role":              binding.Role,
			"cluster":           binding.Cluster,
			"environmentPrefix": binding.EnvironmentPrefix,
			"entraID": map[string]any{
				"appRoleUUID": generateAppRoleUUID(t.GetName(), binding.Role, binding.EnvironmentPrefix),
				"assignment": map[string]any{
					"principalObjectId": binding.PrincipalObjectID,
				},
			},
		})
	}

	values := map[string]any{
		"azure": map[string]any{
			"principalType": azure.PrincipalType,
		},
		"tenant": map[string]any{
			metadataNameKey: t.GetName(),
			"dnsName":       t.Spec.DNSName,
			"argocd": map[string]any{
				"syncRepos": t.SyncRepos,
			},
			"roles":    roles,
			"bindings": renderedBindings,
		},
	}

	valuesYaml, err := yaml.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal gitops values: %w", err)
	}

	app := &argocdtypes.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: argocdtypes.GroupVersion.String(),
			Kind:       argocdtypes.ApplicationKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  "argocd",
			Labels:     commonLabels(t),
			Finalizers: []string{"resources-finalizer.argocd.argoproj.io"},
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
				Name:      "in-cluster",
				Namespace: fmt.Sprintf("gitops-%s", t.GetName()),
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

	return toComposed(app), nil
}
