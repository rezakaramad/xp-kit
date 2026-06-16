package nextinsight

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
// FetchTenantMetadata
// ---------------------------------------------------------------------------

func TestFetchTenantMetadata_HappyPath(t *testing.T) {
	groupResp := groupResponse{}
	groupResp.Data.ID = 44
	groupResp.Data.Name = "Team Falcon"
	groupResp.Data.GroupType.Name = "Agile Team"
	groupResp.Data.ParentGroup.Name = "ART-Platform"

	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/groups/42", func(w http.ResponseWriter, _ *http.Request) {
		serveJSON(t, w, groupResp)
	})

	c, _ := newTestClient(t, mux)

	tenant, err := c.FetchTenantMetadata(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenant.AgileReleaseTrain != "ART-Platform" {
		t.Errorf("AgileReleaseTrain = %q, want %q", tenant.AgileReleaseTrain, "ART-Platform")
	}
	if tenant.AgileTeam != "Team Falcon" {
		t.Errorf("AgileTeam = %q, want %q", tenant.AgileTeam, "Team Falcon")
	}
}

func TestFetchTenantMetadata_GroupsEndpointError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/groups/7", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	c, _ := newTestClient(t, mux)

	_, err := c.FetchTenantMetadata(context.Background(), "7")
	if err == nil {
		t.Fatal("expected error when groups endpoint fails")
	}
	if !strings.Contains(err.Error(), "fetch groups") {
		t.Errorf("error should wrap fetch groups error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FetchTenantLabels
// ---------------------------------------------------------------------------

func TestFetchTenantLabels_HappyPath(t *testing.T) {
	groupResp := groupResponse{}
	groupResp.Data.Name = "Team Falcon"
	groupResp.Data.GroupType.Name = "Agile Team"
	groupResp.Data.ParentGroup.Name = "ART-Platform"

	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/groups/42", func(w http.ResponseWriter, _ *http.Request) {
		serveJSON(t, w, groupResp)
	})

	c, _ := newTestClient(t, mux)

	const prefix = "nextinsight.rezakara.demo/"
	labels, err := c.FetchTenantLabels(context.Background(), "42", prefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if labels[prefix+"agile-release-train"] != "art-platform" {
		t.Errorf("agile-release-train = %q, want %q", labels[prefix+"agile-release-train"], "art-platform")
	}
	if labels[prefix+"agile-team"] != "team-falcon" {
		t.Errorf("agile-team = %q, want %q", labels[prefix+"agile-team"], "team-falcon")
	}
}

// ---------------------------------------------------------------------------
// FetchApplicationMetadata
// ---------------------------------------------------------------------------

func TestFetchApplicationMetadata_HappyPath(t *testing.T) {
	appResp := applicationResponse{}
	appResp.Data.Name = "My Platform App"
	appResp.Data.Lifecycle.Name = "Production"
	appResp.Data.Criticality.Name = "High"
	appResp.Data.DevelopmentType.Name = "In House"
	appResp.Data.FacingInternet = "True"

	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/applications/42", func(w http.ResponseWriter, _ *http.Request) {
		serveJSON(t, w, appResp)
	})

	c, _ := newTestClient(t, mux)

	app, err := c.FetchApplicationMetadata(context.Background(), "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
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
	for _, tc := range checks {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.field, tc.got, tc.want)
		}
	}
}

func TestFetchApplicationMetadata_ApplicationEndpointError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/applications/99", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	c, _ := newTestClient(t, mux)

	_, err := c.FetchApplicationMetadata(context.Background(), "99")
	if err == nil {
		t.Fatal("expected error when application endpoint fails")
	}
	if !strings.Contains(err.Error(), "fetch application") {
		t.Errorf("error should wrap fetch application error, got: %v", err)
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

// ---------------------------------------------------------------------------
// TeamIDExists
// ---------------------------------------------------------------------------

func TestTeamIDExists_HappyPath(t *testing.T) {
	groupResp := groupResponse{}
	groupResp.Data.ID = 42
	groupResp.Data.Name = "Team Falcon"

	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/groups/42", func(w http.ResponseWriter, _ *http.Request) {
		serveJSON(t, w, groupResp)
	})

	c, _ := newTestClient(t, mux)

	if err := c.TeamExists(context.Background(), "42"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestTeamIDExists_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/API/rest/v3/groups/99", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	c, _ := newTestClient(t, mux)

	err := c.TeamExists(context.Background(), "99")
	if err == nil {
		t.Fatal("expected an error for unknown team ID")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error should mention the team ID, got: %v", err)
	}
}
