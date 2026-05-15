package validate

import (
	"context"
	"fmt"
	"strings"

	"github.com/rezakaramad/crossplane-toolkit/modules/nextinsight"
	xtenant "github.com/rezakaramad/crossplane-toolkit/types/xtenant"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidationError represents a validation failure with retry information.
type ValidationError struct {
	Reason    string
	Message   string
	Retryable bool
}

// Deps contains external dependencies required for validation.
type Deps struct {
	Kube                       ctrlclient.Client
	DNS                        DNSClient
	ReferenceEnvironmentDomain string
	// NextInsight is optional; when non-nil, TeamID is verified against the API.
	NextInsight nextinsight.Client
}

// Validate performs full validation of an XTenant.
// It checks DNS availability using the configured referenceEnvironmentDomain,
// and when a TeamID is set, verifies it exists in Next-Insight.
func Validate(ctx context.Context, t xtenant.XTenant, d Deps) *ValidationError {
	if d.NextInsight != nil && t.Spec.TeamID != "" {
		if err := d.NextInsight.TeamExists(ctx, t.Spec.TeamID); err != nil {
			return &ValidationError{
				Reason:    "InvalidTeamID",
				Message:   fmt.Sprintf("spec.teamId %q was not found in Next-Insight", t.Spec.TeamID),
				Retryable: false,
			}
		}
	}

	fqdn := fmt.Sprintf("*.%s.%s.", t.Spec.DNSName, strings.TrimSuffix(d.ReferenceEnvironmentDomain, "."))

	res, err := d.DNS.CheckDNSAvailable(ctx, fqdn)
	if err != nil {
		return &ValidationError{
			Reason:    "DnsCheckFailed",
			Message:   err.Error(),
			Retryable: isRetryable(err),
		}
	}

	if !res.Available {
		return &ValidationError{
			Reason:    "DnsNameTaken",
			Message:   fmt.Sprintf("dns %q already in use", fqdn),
			Retryable: false,
		}
	}

	return nil
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

func isRetryable(err error) bool {
	msg := err.Error()
	return contains(msg, "timeout") ||
		contains(msg, "connection") ||
		contains(msg, "refused")
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
