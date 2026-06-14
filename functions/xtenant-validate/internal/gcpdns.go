package validate

import (
	"context"
	"fmt"

	"github.com/rezakaramad/crossplane-toolkit/modules/gcpdns"
	"google.golang.org/api/option"
)

// NewGCPDNSClient creates a DNSClient backed by Google Cloud DNS.
// Authentication is handled automatically via Workload Identity when running on GKE.
func NewGCPDNSClient(ctx context.Context, project string, opts ...option.ClientOption) (DNSClient, error) {
	c, err := gcpdns.New(ctx, project, opts...)
	if err != nil {
		return nil, fmt.Errorf("create gcp dns client: %w", err)
	}
	return &gcpDNSAdapter{c}, nil
}

// gcpDNSAdapter bridges the gcpdns.Client interface to the local DNSClient interface.
type gcpDNSAdapter struct {
	inner gcpdns.Client
}

func (a *gcpDNSAdapter) CheckDNSAvailable(ctx context.Context, fqdn string) (DNSAvailabilityResult, error) {
	res, err := a.inner.CheckDNSAvailable(ctx, fqdn)
	if err != nil {
		return DNSAvailabilityResult{}, err
	}
	return DNSAvailabilityResult{Available: res.Available, Reason: res.Reason}, nil
}
