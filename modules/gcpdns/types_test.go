package gcpdns

import (
	"testing"
)

func TestBuildFQDN(t *testing.T) {
	cases := []struct {
		name, prefix, base string
		want               string
	}{
		{"pay", "dev", "rezakara.demo", "pay.dev.rezakara.demo."},
		{"pay", "dev", "rezakara.demo.", "pay.dev.rezakara.demo."},
		{"my-app", "wl", "example.com", "my-app.wl.example.com."},
	}

	for _, tc := range cases {
		got := BuildFQDN(tc.name, tc.prefix, tc.base)
		if got != tc.want {
			t.Errorf("BuildFQDN(%q, %q, %q) = %q, want %q",
				tc.name, tc.prefix, tc.base, got, tc.want)
		}
	}
}

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
