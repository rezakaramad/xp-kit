package main

import (
	"context"
	"fmt"
	"maps"

	"github.com/rezakaramad/crossplane-toolkit/modules/nextinsight"

	"github.com/crossplane/function-sdk-go/resource/composed"
)

// nextInsightAppIDLabel is the well-known label key on an XTenant's
// spec.options.labels that carries the Next-Insight application ID.
// Example:
//
//	spec:
//	  options:
//	    labels:
//	      next-insight.io/app-id: "12345"
const nextInsightAppIDLabel = "next-insight.io/app-id"

// fetchNextInsightLabels calls the Next-Insight API for the given appID and
// returns the Kubernetes-safe labels produced by AppMetadata.Labels().
//
// Returns an empty map (no error) when the client is nil or appID is empty —
// so callers can treat Next-Insight enrichment as fully optional without any
// conditionals outside this function.
func fetchNextInsightLabels(ctx context.Context, client nextinsight.Client, appID string) (map[string]string, error) {
	if client == nil || appID == "" {
		return map[string]string{}, nil
	}

	meta, err := client.FetchAppMetadata(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("fetch next-insight metadata for app %q: %w", appID, err)
	}

	return meta.Labels(), nil
}

// applyNextInsightLabels merges the extra labels onto each composed resource.
// It is a no-op when extra is empty, so callers don't need to guard the call.
func applyNextInsightLabels(extra map[string]string, resources ...*composed.Unstructured) {
	if len(extra) == 0 {
		return
	}

	for _, res := range resources {
		if res == nil {
			continue
		}

		existing := res.GetLabels()
		if existing == nil {
			existing = make(map[string]string, len(extra))
		}
		maps.Copy(existing, extra)
		res.SetLabels(existing)
	}
}
