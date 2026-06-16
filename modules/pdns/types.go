package pdns

import (
	"fmt"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// Result represents the availability of a DNS name.
type Result struct {
	Available bool
	Reason    string
}

// EnsureTrailingDot appends a trailing dot to s if it does not already have one.
// PowerDNS and other DNS APIs require FQDNs to be dot-terminated.
func EnsureTrailingDot(s string) string {
	if strings.HasSuffix(s, ".") {
		return s
	}
	return s + "."
}

// extractZone determines the registrable root zone for a given FQDN using
// the public suffix list (eTLD+1). For example, "pay.dev.rezakara.demo."
// returns "rezakara.demo.".
func extractZone(fqdn string) (string, error) {
	trimmed := strings.TrimSuffix(fqdn, ".")
	domain, err := publicsuffix.EffectiveTLDPlusOne(trimmed)
	if err != nil {
		return "", fmt.Errorf("cannot determine zone for fqdn %q: %w", fqdn, err)
	}
	return domain + ".", nil
}
