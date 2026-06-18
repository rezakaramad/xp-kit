// Package pdns provides a DNS availability client backed by the PowerDNS HTTP API.
//
// It exposes a Client interface so callers can inject a fake in tests instead
// of hitting the live API.
//
// The zone to query must be provided explicitly via New(). A tenant name is
// considered taken if any record in the zone equals or is a subdomain of
// "{dnsName}.{zone}" (e.g. "app1.pay.wl.rezakara.demo." marks "pay" as taken).
package pdns

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client checks whether a fully qualified DNS name is available in PowerDNS.
type Client interface {
	// CheckDNSAvailable returns a result indicating whether the given FQDN is
	// free to use. An error is returned only for unexpected API failures —
	// a name that is already taken is not an error; it is expressed via
	// Result.Available == false.
	CheckDNSAvailable(ctx context.Context, fqdn string) (Result, error)
}

// New returns a Client backed by the PowerDNS HTTP API.
// baseURL is the root of the PowerDNS API (e.g. "https://pdns.example.com/api/v1").
// apiKey is the X-Api-Key header value.
// zone is the PowerDNS zone to query (e.g. "wl.rezakara.demo"); a trailing dot is added automatically.
// httpClient is optional; a default client with a 3 s timeout is used when nil.
func New(baseURL, apiKey, zone string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}
	return &pdnsClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		zone:    EnsureTrailingDot(zone),
		client:  httpClient,
	}
}

// pdnsClient is the real implementation of Client.
type pdnsClient struct {
	baseURL string
	apiKey  string
	zone    string
	client  *http.Client
}

// CheckDNSAvailable queries the PowerDNS API to determine whether a fully
// qualified domain name is already registered in the managed zone.
func (c *pdnsClient) CheckDNSAvailable(ctx context.Context, fqdn string) (Result, error) {
	fqdn = EnsureTrailingDot(fqdn)

	// GET /servers/localhost/zones/<zone>
	url := fmt.Sprintf("%s/servers/localhost/zones/%s", c.baseURL, c.zone)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("pdns request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Zone not found → the DNS name is available.
	if resp.StatusCode == http.StatusNotFound {
		return Result{Available: true}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("pdns unexpected status: %d", resp.StatusCode)
	}

	var zoneData struct {
		RRsets []struct {
			Name string `json:"name"`
		} `json:"rrsets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&zoneData); err != nil {
		return Result{}, fmt.Errorf("decode pdns response: %w", err)
	}

	// A tenant is considered taken if any record equals or is a subdomain of the fqdn.
	// e.g. fqdn=pay.wl.rezakara.demo. matches app1.pay.wl.rezakara.demo.
	subdomainSuffix := "." + fqdn
	for _, rr := range zoneData.RRsets {
		if rr.Name == fqdn || strings.HasSuffix(rr.Name, subdomainSuffix) {
			return Result{
				Available: false,
				Reason:    fmt.Sprintf("dns %q already in use", fqdn),
			}, nil
		}
	}

	return Result{Available: true}, nil
}
