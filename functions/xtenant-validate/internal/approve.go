// Package validate implements the business logic for validating XTenant resources.
package validate

import xtenant "github.com/rezakaramad/crossplane-toolkit/types/xtenant"

// IsApproved returns true if the XTenant has been approved.
func IsApproved(t xtenant.XTenant) bool {
	return t.Spec.Approved
}
