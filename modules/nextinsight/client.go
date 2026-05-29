package nextinsight

// Package nextinsight provides a lightweight client for the Next-Insight REST API v3.
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

// Client is the single door into Next-Insight: you knock with an app ID,
// and it hands back everything we need to brand a Kubernetes tenant —
// who owns the app, how critical it is, which ART and team it belongs to.
// Kept as an interface so tests can stand in with a fake instead of hitting the live API.
type Client interface {
	FetchMetadata(ctx context.Context, appID string) (*OwnershipMetadata, *ApplicationMetadata, error)
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

// FetchMetadata calls both the application and groups endpoints and returns
// ownership and application metadata as separate values.
func (c *client) FetchMetadata(ctx context.Context, appID string) (*OwnershipMetadata, *ApplicationMetadata, error) {
	// Get the application details using the /API/rest/v3/applications/{id} endpoint
	app, err := c.fetchApplication(ctx, appID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch application %s: %w", appID, err)
	}

	// Get the groups associated with the application using the /API/rest/v3/applications/{id}/groups endpoint
	groups, err := c.fetchGroups(ctx, appID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch groups for application %s: %w", appID, err)
	}

	return buildOwnershipMetadata(groups), buildApplicationMetadata(appID, app), nil
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

// JSON shape we expect back from the API for this endpoint
// /API/rest/v3/applications/{id}/groups
type groupsResponse struct {
	Data []groupItem `json:"data"`
}

// JSON shape we expect back from the API for each group item in the groupsResponse
// Matches the "groups" array items in the response from /API/rest/v3/applications/{id}/groups
type groupItem struct {
	Name string `json:"name"`
	Type string `json:"type"`
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

// Gets groups associated with an application from Next-Insight and converts the JSON response into a Go struct.
func (c *client) fetchGroups(ctx context.Context, appID string) (*groupsResponse, error) {
	// Build the URL for the groups endpoint and make the GET request
	// E.g. GET https://app.next-insight.com/API/rest/v3/applications/12345/groups
	url := fmt.Sprintf("%s/API/rest/v3/applications/%s/groups", c.baseURL, appID)
	var out groupsResponse
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

// The group types in Next-Insight that we care about for metadata purposes;
// Used to filter the groups associated with an application into ARTs and Agile Teams when building AppMetadata.
const (
	groupTypeART       = "Agile Release Train"
	groupTypeAgileTeam = "Agile Team"
)

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

// buildOwnershipMetadata projects the groups response into an OwnershipMetadata.
// Keeps only the first ART and Agile Team to stay deterministic.
func buildOwnershipMetadata(groups *groupsResponse) *OwnershipMetadata {
	metadata := &OwnershipMetadata{}
	for _, group := range groups.Data {
		switch group.Type {
		case groupTypeART:
			if metadata.AgileReleaseTrain == "" {
				metadata.AgileReleaseTrain = group.Name
			}
		case groupTypeAgileTeam:
			if metadata.AgileTeam == "" {
				metadata.AgileTeam = group.Name
			}
		}
	}
	return metadata
}
