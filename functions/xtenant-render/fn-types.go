package main

import xtenant "github.com/rezakaramad/crossplane-toolkit/types/xtenant"

const (
	defaultCrossplaneNamespace = "crossplane"
	managedByCrossplane        = "crossplane"
	metadataNameKey            = "name"
)

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
