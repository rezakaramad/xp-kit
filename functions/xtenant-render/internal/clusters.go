package render

import (
	inputv1beta1 "github.com/rezakaramad/crossplane-toolkit/functions/xtenant-render/input/v1beta1"
)

// UniqueClustersFromBindings deduplicates bindings into a slice of unique clusters.
func UniqueClustersFromBindings(bindings []inputv1beta1.BindingInput) []Cluster {
	clusters := make([]Cluster, 0, len(bindings))
	seen := make(map[string]struct{}, len(bindings))

	for _, binding := range bindings {
		key := binding.Cluster + "/" + binding.EnvironmentPrefix
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		clusters = append(clusters, Cluster{Name: binding.Cluster, Prefix: binding.EnvironmentPrefix})
	}

	return clusters
}
