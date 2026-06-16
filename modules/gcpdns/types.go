package gcpdns

import (
	"fmt"
	"strings"
)

// Result represents the availability of a DNS name.
type Result struct {
	Available bool
	Reason    string
}

// BuildFQDN constructs a tenant FQDN from its components.
// For example: BuildFQDN("pay", "dev", "rezakara.demo") → "pay.dev.rezakara.demo."
func BuildFQDN(name, prefix, base string) string {
	base = strings.TrimSuffix(base, ".")
	return fmt.Sprintf("%s.%s.%s.", name, prefix, base)
}

// EnsureTrailingDot appends a trailing dot to s if it does not already have one.
// Cloud DNS and other DNS APIs require FQDNs to be dot-terminated.
func EnsureTrailingDot(s string) string {
	if strings.HasSuffix(s, ".") {
		return s
	}
	return s + "."
}
