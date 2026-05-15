package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"
)

// This PowerDNS implementation assumes tenant DNS records live in the
// registrable root zone (for example, "rezakara.demo.").
// It does not discover the most specific delegated zone such as
// "dev.rezakara.demo.".

// pdnsClient implements DNSClient by calling the PowerDNS HTTP API.
type pdnsClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewPowerDNSClient returns a DNSClient backed by the PowerDNS API.
func NewPowerDNSClient(baseURL, apiKey string, httpClient *http.Client) DNSClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 3 * time.Second}
	}
	return &pdnsClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		client:  httpClient,
	}
}

// CheckDNSAvailable queries the PowerDNS API to determine whether a fully
// qualified domain name is already registered in the managed zone.
func (c *pdnsClient) CheckDNSAvailable(ctx context.Context, fqdn string) (DNSAvailabilityResult, error) {
	// PowerDNS API expects FQDNs to have a trailing dot. Ensure it is present.
	fqdn = ensureTrailingDot(fqdn)

	// Extract the zone from the FQDN. For example, if fqdn is "pay.dev.rezakara.demo.",
	// the zone would be "rezakara.demo.". This is needed to construct the correct API URL.
	zone, err := extractZone(fqdn)
	if err != nil {
		return DNSAvailabilityResult{}, err
	}

	// Construct the API URL to list resource record sets in the zone. For example:
	// https://pdns.example.com/api/v1/servers/localhost/zones/rezakara.demo.
	url := fmt.Sprintf("%s/servers/localhost/zones/%s", c.baseURL, zone)

	// Build the HTTP request with the appropriate headers for authentication.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return DNSAvailabilityResult{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Api-Key", c.apiKey)

	// Send the request to the PowerDNS API.
	resp, err := c.client.Do(req)
	if err != nil {
		return DNSAvailabilityResult{}, fmt.Errorf("pdns request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Zone not found → the DNS name is available.
	if resp.StatusCode == http.StatusNotFound {
		return DNSAvailabilityResult{Available: true}, nil
	}

	// Any other non-200 status code is an error.
	if resp.StatusCode != http.StatusOK {
		return DNSAvailabilityResult{}, fmt.Errorf("pdns unexpected status: %d", resp.StatusCode)
	}

	// PowerDNS returns a JSON object with an "rrsets" field which is a list
	// Each item in that list has "name" and "type" fields
	// Read JSON from the response, match JSON fields to this struct and store results in zoneData
	// E.g., PowerDNS might return:
	// {
	//   "rrsets": [
	//     {"name": "pay.dev.rezakara.demo.", "type": "A"},
	//     {"name": "www.pay.dev.rezakara.demo.", "type": "CNAME"}
	//   ]
	// }
	// after decoding, zoneData will be:
	// zoneData = {
	//   RRsets: [
	//     {Name: "pay.dev.rezakara.demo.", Type: "A"},
	//     {Name: "www.pay.dev.rezakara.demo.", Type: "CNAME"}
	//   ]
	// }
	var zoneData struct {
		RRsets []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"rrsets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&zoneData); err != nil {
		return DNSAvailabilityResult{}, fmt.Errorf("decode pdns response: %w", err)
	}

	// Loop through the list of record sets and look for an exact match.
	// If found, the DNS name is not available. If not found, it is available.
	for _, rr := range zoneData.RRsets {
		if rr.Name == fqdn {
			return DNSAvailabilityResult{
				Available: false,
				Reason:    fmt.Sprintf("dns %q already exists", fqdn),
			}, nil
		}
	}

	// No exact match found → the DNS name is available.
	return DNSAvailabilityResult{Available: true}, nil
}

// ---------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------

// BuildFQDN constructs a tenant FQDN like: pay.dev.rezakara.demo.
// Input:
// - name: the tenant's chosen DNS name (e.g. "pay").
// - prefix: the cluster prefix (e.g. "dev").
// - base: the base domain (e.g. "rezakara.demo").
// Output:
// - The fully qualified domain name for the tenant (e.g. "pay.dev.rezakara.demo.").
func BuildFQDN(name, prefix, base string) string {
	base = strings.TrimSuffix(base, ".")
	return fmt.Sprintf("%s.%s.%s.", name, prefix, base)
}

// Determines the zone for a given FQDN by extracting the effective top-level domain plus one (eTLD+1).
// Input:
// - fqdn: a fully qualified domain name (e.g. "pay.dev.rezakara.demo.").
// Output:
// - The zone for the FQDN (e.g. "rezakara.demo.") or an error if the zone cannot be determined.
func extractZone(fqdn string) (string, error) {
	trimmed := strings.TrimSuffix(fqdn, ".")
	domain, err := publicsuffix.EffectiveTLDPlusOne(trimmed)
	if err != nil {
		return "", fmt.Errorf("cannot determine zone for fqdn %q: %w", fqdn, err)
	}
	return domain + ".", nil
}

// Ensures that the given string has a trailing dot, which is required for FQDNs in DNS APIs.
// Input:
// - s: a string representing a domain name (e.g. "rezakara.demo").
// Output:
// - The input string with a trailing dot (e.g. "rezakara.demo.").
func ensureTrailingDot(s string) string {
	if strings.HasSuffix(s, ".") {
		return s
	}
	return s + "."
}
