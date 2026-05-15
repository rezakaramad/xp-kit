package pdns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, handler http.Handler) (*pdnsClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &pdnsClient{
		baseURL: srv.URL,
		apiKey:  "test-key",
		client:  srv.Client(),
	}, srv
}

func TestCheckDNSAvailable_Available(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Errorf("missing or wrong X-Api-Key header: %q", r.Header.Get("X-Api-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rrsets": []map[string]any{
				{"name": "other.example.com."},
			},
		})
	}))

	res, err := c.CheckDNSAvailable(context.Background(), "pay.example.com.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Available {
		t.Errorf("expected Available=true, got false: %s", res.Reason)
	}
}

func TestCheckDNSAvailable_Taken(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"rrsets": []map[string]any{
				{"name": "pay.example.com."},
			},
		})
	}))

	res, err := c.CheckDNSAvailable(context.Background(), "pay.example.com.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Available {
		t.Error("expected Available=false, got true")
	}
}

func TestCheckDNSAvailable_ZoneNotFound(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	res, err := c.CheckDNSAvailable(context.Background(), "pay.example.com.")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Available {
		t.Errorf("expected Available=true when zone not found, got false: %s", res.Reason)
	}
}

func TestCheckDNSAvailable_APIError(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	_, err := c.CheckDNSAvailable(context.Background(), "pay.example.com.")
	if err == nil {
		t.Error("expected error for non-200/404 status, got nil")
	}
}
