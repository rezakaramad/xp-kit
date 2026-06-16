package validate

import (
	xtenant "github.com/rezakaramad/xp-kit/types/xtenant"

	"github.com/crossplane/function-sdk-go/resource"
)

// SetPhase writes status.phase onto the XR so Crossplane surfaces it.
func SetPhase(xr *resource.Composite, phase xtenant.Phase) {
	_ = xr.Resource.SetValue("status.phase", string(phase))
}
