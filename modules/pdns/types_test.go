package pdns

import (
	"testing"
)

func TestEnsureTrailingDot(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"rezakara.demo", "rezakara.demo."},
		{"rezakara.demo.", "rezakara.demo."},
		{"", "."},
	}

	for _, tc := range cases {
		got := EnsureTrailingDot(tc.input)
		if got != tc.want {
			t.Errorf("EnsureTrailingDot(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExtractZone(t *testing.T) {
	cases := []struct {
		fqdn    string
		want    string
		wantErr bool
	}{
		{"pay.dev.rezakara.demo.", "rezakara.demo.", false},
		{"pay.dev.rezakara.demo", "rezakara.demo.", false},
		{"app.example.com.", "example.com.", false},
	}

	for _, tc := range cases {
		got, err := extractZone(tc.fqdn)
		if (err != nil) != tc.wantErr {
			t.Errorf("extractZone(%q) error = %v, wantErr %v", tc.fqdn, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("extractZone(%q) = %q, want %q", tc.fqdn, got, tc.want)
		}
	}
}
