// Package pdns provides a DNS availability client backed by the PowerDNS HTTP API.
//
// It exposes a Client interface so callers can inject a fake in tests instead
// of hitting the live API.
//
// This implementation assumes tenant DNS records live in the registrable root
// zone (for example, "rezakara.demo."). It does not discover the most specific
// delegated zone such as "dev.rezakara.demo.".
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
// httpClient is optional; a default client with a 3 s timeout is used when nil.
func New(baseURL, apiKey string, httpClient *http.Client) Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}
	return &pdnsClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		client:  httpClient,
	}
}

// pdnsClient is the real implementation of Client.
type pdnsClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// CheckDNSAvailable queries the PowerDNS API to determine whether a fully
// qualified domain name is already registered in the managed zone.
func (c *pdnsClient) CheckDNSAvailable(ctx context.Context, fqdn string) (Result, error) {
	fqdn = EnsureTrailingDot(fqdn)

	zone, err := extractZone(fqdn)
	if err != nil {
		return Result{}, err
	}

	// GET /servers/localhost/zones/<zone>
	url := fmt.Sprintf("%s/servers/localhost/zones/%s", c.baseURL, zone)

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

	for _, rr := range zoneData.RRsets {
		if rr.Name == fqdn {
			return Result{
				Available: false,
				Reason:    fmt.Sprintf("dns %q already exists", fqdn),
			}, nil
		}
	}

	return Result{Available: true}, nil
}
