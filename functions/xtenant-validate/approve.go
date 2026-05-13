package main

import xtenant "github.com/rezakaramad/crossplane-toolkit/types/xtenant"

// IsApproved returns true if the XTenant has been approved.
func IsApproved(t xtenant.XTenant) bool {
	return t.Spec.Approved
}
