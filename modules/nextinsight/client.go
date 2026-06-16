// Package nextinsight provides a lightweight client for the Next-Insight REST API v3.
package nextinsight

// It fetches application metadata and group associations and assembles them
// into an 'AppMetadata' value that can be stamped onto Kubernetes resource
// labels and annotations.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client is the interface for querying Next-Insight metadata.
// Kept as an interface so tests can stand in with a fake instead of hitting the live API.
type Client interface {
	// FetchTenantMetadata returns ART and Agile Team ownership for the given
	// application ID by calling the /groups endpoint.
	FetchTenantMetadata(ctx context.Context, appID string) (*TenantMetadata, error)

	// FetchTenantLabels returns Kubernetes labels derived from tenant ownership
	// metadata, ready to stamp onto Namespace-boundary resources.
	// It is the single call a render function needs — no intermediate struct required.
	FetchTenantLabels(ctx context.Context, appID, labelPrefix string) (map[string]string, error)

	// FetchApplicationMetadata returns application classification data for the
	// given application ID by calling the /applications/{id} endpoint.
	FetchApplicationMetadata(ctx context.Context, appID string) (*ApplicationMetadata, error)

	// TeamExists returns nil when the given ID resolves to a known entry in
	// Next-Insight, or an error if it does not exist or the API is unreachable.
	TeamExists(ctx context.Context, teamID string) error
}

// New returns a Client that calls the Next-Insight API at baseURL using the
// supplied bearer token. baseURL should be the root of the API, e.g.
// "https://app.next-insight.com".
func New(baseURL, token string) Client {
	return &client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// The real implementation of Client that calls the Next-Insight API over HTTP.
type client struct {
	baseURL string
	token   string
	http    *http.Client
}

// FetchTenantMetadata calls the /groups/{id} endpoint and returns ART and Agile Team ownership.
func (c *client) FetchTenantMetadata(ctx context.Context, teamID string) (*TenantMetadata, error) {
	group, err := c.fetchGroup(ctx, teamID)
	if err != nil {
		return nil, fmt.Errorf("fetch groups for application %s: %w", teamID, err)
	}
	return buildOwnershipMetadata(group), nil
}

// FetchTenantLabels returns Kubernetes labels derived from tenant ownership metadata.
func (c *client) FetchTenantLabels(ctx context.Context, appID, labelPrefix string) (map[string]string, error) {
	meta, err := c.FetchTenantMetadata(ctx, appID)
	if err != nil {
		return nil, err
	}
	return meta.TenantLabels(labelPrefix), nil
}

// FetchApplicationMetadata calls the /applications/{id} endpoint and returns application classification data.
func (c *client) FetchApplicationMetadata(ctx context.Context, appID string) (*ApplicationMetadata, error) {
	app, err := c.fetchApplication(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("fetch application %s: %w", appID, err)
	}
	return buildApplicationMetadata(appID, app), nil
}

// TeamExists returns nil when the given ID resolves to a known group in Next-Insight.
// It calls the /groups/{id} endpoint — a successful response confirms the ID is valid.
func (c *client) TeamExists(ctx context.Context, teamID string) error {
	_, err := c.fetchGroup(ctx, teamID)
	if err != nil {
		return fmt.Errorf("team %q not found in Next-Insight: %w", teamID, err)
	}
	return nil
}

// JSON shape we expect back from the API for this endpoint
// /API/rest/v3/applications/{id}
type applicationResponse struct {
	Data struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Lifecycle struct {
			Name string `json:"name"`
		} `json:"lifecycle"`
		LifecycleDecision struct {
			Name string `json:"name"`
		} `json:"lifecycleDecision"`
		Criticality struct {
			Name string `json:"name"`
		} `json:"criticality"`
		Complexity struct {
			Name string `json:"name"`
		} `json:"complexity"`
		PrimaryCategory struct {
			Name string `json:"name"`
		} `json:"primaryCategory"`
		DevelopmentType struct {
			Name string `json:"name"`
		} `json:"developmentType"`
		SourcingType struct {
			Name string `json:"name"`
		} `json:"sourcingType"`
		SubType struct {
			Name string `json:"name"`
		} `json:"subType"`
		FacingInternet string `json:"facingInternet"`
		PersonalData   string `json:"personalData"`
		BusinessFit    string `json:"businessFit"`
		TechnicalFit   string `json:"technicalFit"`
	} `json:"data"`
}

// JSON shape returned by GET /API/rest/v3/groups/{id}
type groupResponse struct {
	Data struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		GroupType struct {
			Name string `json:"name"`
		} `json:"groupType"`
		ParentGroup struct {
			Name string `json:"name"`
		} `json:"parentGroup"`
	} `json:"data"`
}

// Gets an application by ID from Next-Insight and coverts the JSON response into a Go struct.
func (c *client) fetchApplication(ctx context.Context, appID string) (*applicationResponse, error) {
	// Build the URL for the application endpoint and make the GET request
	// E.g. GET https://app.next-insight.com/API/rest/v3/applications/12345
	url := fmt.Sprintf("%s/API/rest/v3/applications/%s", c.baseURL, appID)
	// The response is expected to be a JSON object matching the applicationResponse struct
	var out applicationResponse
	if err := c.get(ctx, url, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Gets a group by ID from Next-Insight and converts the JSON response into a Go struct.
func (c *client) fetchGroup(ctx context.Context, groupID string) (*groupResponse, error) {
	// E.g. GET https://app.next-insight.com/API/rest/v3/groups/44
	url := fmt.Sprintf("%s/API/rest/v3/groups/%s", c.baseURL, groupID)
	var out groupResponse
	if err := c.get(ctx, url, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Performs an HTTP GET request to the specified URL with the appropriate headers and decodes the JSON response into the provided output struct.
func (c *client) get(ctx context.Context, url string, out any) error {
	// Build the HTTP request with the context, URL, and headers (including the bearer token for authentication)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	// Set the Authorization header with the bearer token for authentication
	req.Header.Set("Authorization", "Bearer "+c.token)
	// The API returns JSON responses, so we set the Accept header accordingly
	req.Header.Set("Accept", "application/json")

	// Perform the HTTP request
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	// Ensure the response body is closed after we're done with it to prevent resource leaks
	defer func() { _ = resp.Body.Close() }()

	// Check if the response status code is 200 OK; if not, return an error
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	// Decode the JSON response body into the provided output struct
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// buildApplicationMetadata projects the application response into an ApplicationMetadata.
func buildApplicationMetadata(appID string, app *applicationResponse) *ApplicationMetadata {
	return &ApplicationMetadata{
		ApplicationID:     appID,
		ApplicationName:   app.Data.Name,
		Lifecycle:         app.Data.Lifecycle.Name,
		LifecycleDecision: app.Data.LifecycleDecision.Name,
		Criticality:       app.Data.Criticality.Name,
		Complexity:        app.Data.Complexity.Name,
		Category:          app.Data.PrimaryCategory.Name,
		DevelopmentType:   app.Data.DevelopmentType.Name,
		SourcingType:      app.Data.SourcingType.Name,
		FacingInternet:    strings.ToLower(app.Data.FacingInternet),
	}
}

// buildOwnershipMetadata projects the group response into a TenantMetadata.
// The group itself is the Agile Team; its parent group is the ART.
func buildOwnershipMetadata(group *groupResponse) *TenantMetadata {
	return &TenantMetadata{
		AgileTeam:         group.Data.Name,
		AgileReleaseTrain: group.Data.ParentGroup.Name,
	}
}
