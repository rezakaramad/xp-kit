package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

// gcpDNSClient implements DNSClient against the Google Cloud DNS API.
//
// Authentication is handled automatically via Workload Identity when the pod
// runs on GKE — no service account key file is needed. The GCP client library
// contacts the GKE metadata server to obtain a short-lived token scoped to the
// Kubernetes service account that is bound to a GCP service account.
type gcpDNSClient struct {
	project string       // The GCP project that owns the Cloud DNS managed zone.
	svc     *dns.Service // SDK client for calling the Cloud DNS API. Shared across calls to take advantage of HTTP connection pooling.
}

// NewGCPDNSClient creates a DNSClient backed by Google Cloud DNS.
// Inputs:
// - ctx: the context for API calls.
// - project: the GCP project that owns the Cloud DNS managed zone.
// - opts: additional options for configuring the GCP client (e.g. credentials). When running on GKE with Workload Identity, no options are needed.
// Output:
// - A DNSClient that can be used to check DNS availability by querying the Cloud DNS API.
func NewGCPDNSClient(ctx context.Context, project string, opts ...option.ClientOption) (DNSClient, error) {
	// Create a new Cloud DNS service client using the provided context and options.
	// The client will automatically use Workload Identity credentials when running on GKE,
	// so no special handling is needed for authentication.
	svc, err := dns.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create gcp dns service: %w", err)
	}
	return &gcpDNSClient{project: project, svc: svc}, nil
}

// CheckDNSAvailable queries the Google Cloud DNS API to determine whether a fully
// qualified domain name is already registered in the managed zone.
func (c *gcpDNSClient) CheckDNSAvailable(ctx context.Context, fqdn string) (DNSAvailabilityResult, error) {
	// Cloud DNS API expects FQDNs to have a trailing dot. Ensure it is present.
	fqdn = ensureTrailingDot(fqdn)

	// Cloud DNS zone names use the registered domain as the DNS name.
	// We list all zones and find the one whose dnsName is a suffix of the fqdn.
	zone, err := c.findZone(ctx, fqdn)
	if err != nil {
		return DNSAvailabilityResult{}, err
	}

	// If no zone exists for this domain, the name is available.
	if zone == "" {
		return DNSAvailabilityResult{Available: true}, nil
	}

	// List resource record sets in the zone and look for an exact match.
	req := c.svc.ResourceRecordSets.List(c.project, zone).
		Name(fqdn).
		Context(ctx)

	// Iterates through the pages of results from the Cloud DNS API.
	// For each page, it checks if any of the resource record sets have a name that matches the fqdn.
	// If a match is found, it returns errFound to stop pagination early.
	// If no matches are found, it means the DNS name is available.
	err = req.Pages(ctx, func(page *dns.ResourceRecordSetsListResponse) error {
		for _, rr := range page.Rrsets {
			if strings.EqualFold(rr.Name, fqdn) {
				return errFound
			}
		}
		return nil
	})

	// If the error is errFound, it means we found a record with the same name → the DNS name is taken.
	if errors.Is(err, errFound) {
		return DNSAvailabilityResult{
			Available: false,
			Reason:    fmt.Sprintf("dns %q already exists", fqdn),
		}, nil
	}
	if err != nil {
		return DNSAvailabilityResult{}, fmt.Errorf("list cloud dns records: %w", err)
	}

	// No exact match found → the DNS name is available.
	return DNSAvailabilityResult{Available: true}, nil
}

// Returns the managed zone resource name for a zone whose DnsName
// is a suffix of fqdn, or an empty string if no matching zone exists.
// E.g., if fqdn is "pay.dev.rezakara.demo." and a managed zone exists
// with DnsName "dev.rezakara.demo.", this function may return that zone's
// resource name, such as "dev-zone".
// This function keeps scanning all zones and chooses the longest matching suffix instead of the first one.
func (c *gcpDNSClient) findZone(ctx context.Context, fqdn string) (string, error) {
	var match string
	// Get the list of managed zones in the project
	err := c.svc.ManagedZones.List(c.project).Context(ctx).Pages(ctx,
		// Iterates through the pages of results from the Cloud DNS API.
		func(page *dns.ManagedZonesListResponse) error {
			// for each page, it checks if any of the managed zones have a DnsName that is a suffix of the fqdn.
			for _, z := range page.ManagedZones {
				// If a match is found, it stores the zone's resource name.
				if strings.HasSuffix(fqdn, z.DnsName) {
					match = z.Name
					return nil
				}
			}
			return nil
		},
	)
	if err != nil {
		return "", fmt.Errorf("list cloud dns zones: %w", err)
	}
	return match, nil
}

// errFound is a sentinel used to stop pagination early when a match is found.
var errFound = fmt.Errorf("record found")
