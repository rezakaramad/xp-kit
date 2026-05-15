package validate

import (
	"context"
	"net/http"

	"github.com/rezakaramad/crossplane-toolkit/modules/pdns"
)

// pdnsAdapter bridges pdns.Client to the local DNSClient interface.
type pdnsAdapter struct {
	inner pdns.Client
}

func (a *pdnsAdapter) CheckDNSAvailable(ctx context.Context, fqdn string) (DNSAvailabilityResult, error) {
	res, err := a.inner.CheckDNSAvailable(ctx, fqdn)
	if err != nil {
		return DNSAvailabilityResult{}, err
	}
	return DNSAvailabilityResult{Available: res.Available, Reason: res.Reason}, nil
}

// NewPowerDNSClient creates a DNSClient backed by the PowerDNS HTTP API.
func NewPowerDNSClient(baseURL, apiKey string, httpClient *http.Client) DNSClient {
	return &pdnsAdapter{pdns.New(baseURL, apiKey, httpClient)}
}
