package main

import (
	"context"
	"fmt"
	"strings"

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
	Kube             ctrlclient.Client
	DNS              DNSClient
	BaseDomain       string
	WorkloadClusters []xtenant.Cluster
}

// Validate performs full validation of an XTenant.
// It checks DNS availability for every workload cluster the tenant targets.
func Validate(ctx context.Context, t xtenant.XTenant, d Deps) *ValidationError {
	dnsName := t.Spec.DNSName

	for _, cluster := range d.WorkloadClusters {
		fqdn := BuildFQDN(dnsName, cluster.Prefix, d.BaseDomain)

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
