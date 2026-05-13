package main

import "context"

// DNSClient checks whether a fully qualified DNS name is available.
type DNSClient interface {
	CheckDNSAvailable(ctx context.Context, fqdn string) (DNSAvailabilityResult, error)
}

// DNSAvailabilityResult represents the availability of a DNS name.
type DNSAvailabilityResult struct {
	Available bool
	Reason    string
}
