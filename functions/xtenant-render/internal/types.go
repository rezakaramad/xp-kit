package render

import xtenant "github.com/rezakaramad/crossplane-toolkit/types/xtenant"

const (
	managedByCrossplane = "crossplane"
	metadataNameKey     = "name"
)

// Cluster identifies a workload cluster and its environment prefix.
type Cluster struct {
	Name   string
	Prefix string
}

// TenantSpec is the renderer's internal view of an XTenant.
// It embeds xtenant.XTenant so all XR fields are accessible directly,
// and adds renderer-only fields that have no representation in the XR schema.
type TenantSpec struct {
	xtenant.XTenant

	// SyncRepos is derived at render time from the tenant name, not read from the XR.
	SyncRepos []string
}

func commonLabels(t TenantSpec) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by":  managedByCrossplane,
		"platform.rezakara.demo/tenant": t.GetName(),
	}
}
