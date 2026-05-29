package nextinsight

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// roundTripFunc lets us swap the real HTTP transport with a function in tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// newTestClient wires a *client against a fake httptest.Server and returns
// both so tests can assert on the requests that were made.
func newTestClient(t *testing.T, handler http.Handler) (*client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &client{
		baseURL: srv.URL,
		token:   "test-token",
		http:    srv.Client(),
	}, srv
}

// serveJSON writes v as a JSON body with status 200.
func serveJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("serveJSON: %v", err)
	}
}

// ---------------------------------------------------------------------------
// get — low-level HTTP helper
// ---------------------------------------------------------------------------

func TestGet_SetsAuthAndAcceptHeaders(t *testing.T) {
	var gotAuth, gotAccept string

	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		serveJSON(t, w, map[string]any{})
	}))

	var out map[string]any
	if err := c.get(context.Background(), c.baseURL+"/test", &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept header = %q, want %q", gotAccept, "application/json")
	}
}

func TestGet_ErrorOnNon200(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	var out map[string]any
	err := c.get(context.Background(), c.baseURL+"/missing", &out)
	if err == nil {
		t.Fatal("expected an error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status code, got: %v", err)
	}
}

func TestGet_ErrorOnInvalidJSON(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))

	var out map[string]any
	err := c.get(context.Background(), c.baseURL+"/bad-json", &out)
	if err == nil {
		t.Fatal("expected a decode error, got nil")
	}
}

// ---------------------------------------------------------------------------
// FetchMetadata — integration through the full client stack
// ---------------------------------------------------------------------------

func TestFetchMetadata_HappyPath(t *testing.T) {
	appResp := applicationResponse{}
	appResp.Data.Name = "My Platform App"
	appResp.Data.Lifecycle.Name = "Production"
	appResp.Data.Criticality.Name = "High"
	appResp.Data.DevelopmentType.Name = "In House"
	appResp.Data.FacingInternet = "True"

	groupsResp := groupsResponse{
		Data: []groupItem{
			{Name: "ART-Platform", Type: groupTypeART},
			{Name: "Team Falcon", Type: groupTypeAgileTeam},
			// second ART — should be ignored (first-wins)
			{Name: "ART-Other", Type: groupTypeART},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/applications/42", func(w http.ResponseWriter, _ *http.Request) {
		serveJSON(t, w, appResp)
	})
	mux.HandleFunc("/API/rest/v3/applications/42/groups", func(w http.ResponseWriter, _ *http.Request) {
		serveJSON(t, w, groupsResp)
	})

	c, _ := newTestClient(t, mux)

	ownership, app, err := c.FetchMetadata(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ownership fields
	if ownership.AgileReleaseTrain != "ART-Platform" {
		t.Errorf("AgileReleaseTrain = %q, want %q", ownership.AgileReleaseTrain, "ART-Platform")
	}
	if ownership.AgileTeam != "Team Falcon" {
		t.Errorf("AgileTeam = %q, want %q", ownership.AgileTeam, "Team Falcon")
	}

	// Application fields
	appChecks := []struct {
		field string
		got   string
		want  string
	}{
		{"ApplicationID", app.ApplicationID, "42"},
		{"ApplicationName", app.ApplicationName, "My Platform App"},
		{"Lifecycle", app.Lifecycle, "Production"},
		{"Criticality", app.Criticality, "High"},
		{"DevelopmentType", app.DevelopmentType, "In House"},
		{"FacingInternet", app.FacingInternet, "true"},
	}
	for _, tc := range appChecks {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, tc.got, tc.want)
		}
	}
}

func TestFetchMetadata_ApplicationEndpointError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/applications/99", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	c, _ := newTestClient(t, mux)

	_, _, err := c.FetchMetadata(context.Background(), "99")
	if err == nil {
		t.Fatal("expected error when application endpoint fails")
	}
	if !strings.Contains(err.Error(), "fetch application") {
		t.Errorf("error message should wrap fetch application error, got: %v", err)
	}
}

func TestFetchMetadata_GroupsEndpointError(t *testing.T) {
	appResp := applicationResponse{}
	appResp.Data.Name = "Some App"

	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/applications/7", func(w http.ResponseWriter, _ *http.Request) {
		serveJSON(t, w, appResp)
	})
	mux.HandleFunc("/API/rest/v3/applications/7/groups", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	c, _ := newTestClient(t, mux)

	_, _, err := c.FetchMetadata(context.Background(), "7")
	if err == nil {
		t.Fatal("expected error when groups endpoint fails")
	}
	if !strings.Contains(err.Error(), "fetch groups") {
		t.Errorf("error message should wrap fetch groups error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// New — constructor strips trailing slash from baseURL
// ---------------------------------------------------------------------------

func TestNew_StripsTrailingSlash(t *testing.T) {
	c := New("https://app.next-insight.com/", "tok").(*client)
	if strings.HasSuffix(c.baseURL, "/") {
		t.Errorf("baseURL should not have trailing slash, got %q", c.baseURL)
	}
}
