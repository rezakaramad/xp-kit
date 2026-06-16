// Package gcpdns provides a DNS availability client backed by Google Cloud DNS.
//
// It exposes a Client interface so callers can inject a fake in tests instead
// of hitting the live API.
//
// Authentication is handled automatically via Workload Identity when the pod
// runs on GKE — no service account key file is needed.
package gcpdns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
)

// Client checks whether a fully qualified DNS name is available in Google Cloud DNS.
type Client interface {
	// CheckDNSAvailable returns a result indicating whether the given FQDN is
	// free to use. An error is returned only for unexpected API failures —
	// a name that is already taken is not an error; it is expressed via
	// Result.Available == false.
	CheckDNSAvailable(ctx context.Context, fqdn string) (Result, error)
}

// New creates a Client backed by Google Cloud DNS.
// project is the GCP project that owns the Cloud DNS managed zone.
// opts are passed directly to the underlying GCP client; when running on GKE
// with Workload Identity no extra options are needed.
func New(ctx context.Context, project string, opts ...option.ClientOption) (Client, error) {
	svc, err := dns.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create gcp dns service: %w", err)
	}
	return &gcpClient{project: project, svc: svc}, nil
}

// gcpClient is the real implementation of Client.
type gcpClient struct {
	project string
	svc     *dns.Service
}

// CheckDNSAvailable queries the Google Cloud DNS API to determine whether a
// fully qualified domain name is already registered in the managed zone.
func (c *gcpClient) CheckDNSAvailable(ctx context.Context, fqdn string) (Result, error) {
	// Cloud DNS API expects FQDNs to have a trailing dot.
	fqdn = EnsureTrailingDot(fqdn)

	// Find the managed zone whose dnsName is the longest suffix of fqdn.
	zone, err := c.findZone(ctx, fqdn)
	if err != nil {
		return Result{}, err
	}

	// No zone exists for this domain — the name is available.
	if zone == "" {
		return Result{Available: true}, nil
	}

	// List resource record sets in the zone and look for an exact match.
	req := c.svc.ResourceRecordSets.List(c.project, zone).
		Name(fqdn).
		Context(ctx)

	err = req.Pages(ctx, func(page *dns.ResourceRecordSetsListResponse) error {
		for _, rr := range page.Rrsets {
			if strings.EqualFold(rr.Name, fqdn) {
				return errFound
			}
		}
		return nil
	})

	if errors.Is(err, errFound) {
		return Result{
			Available: false,
			Reason:    fmt.Sprintf("dns %q already exists", fqdn),
		}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("list cloud dns records: %w", err)
	}

	return Result{Available: true}, nil
}

// findZone returns the managed zone resource name whose DnsName is a suffix of
// fqdn, choosing the longest match. Returns an empty string when no zone matches.
func (c *gcpClient) findZone(ctx context.Context, fqdn string) (string, error) {
	var (
		match       string
		matchLength int
	)

	err := c.svc.ManagedZones.List(c.project).Context(ctx).Pages(ctx,
		func(page *dns.ManagedZonesListResponse) error {
			for _, z := range page.ManagedZones {
				if strings.HasSuffix(fqdn, z.DnsName) && len(z.DnsName) > matchLength {
					match = z.Name
					matchLength = len(z.DnsName)
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
var errFound = errors.New("record found")
