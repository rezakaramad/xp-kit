package main

import "github.com/crossplane/function-sdk-go/resource"

// Phase constants mirror the XTenant Phase type so the function
// can set status.phase without importing the full types module.
const (
	PhaseValidating      = "Validating"
	PhasePendingApproval = "PendingApproval"
	PhaseProvisioning    = "Provisioning"
	PhaseReady           = "Ready"
	PhaseFailed          = "Failed"
)

// SetPhase writes status.phase onto the XR so Crossplane surfaces it.
func SetPhase(xr *resource.Composite, phase string) {
	_ = xr.Resource.SetValue("status.phase", phase)
}
