package nextinsight

import (
	"regexp"
	"strings"
)

// set writes key=value into m only when value is non-empty.
func set(m map[string]string, key, value string) {
	if value != "" {
		m[key] = value
	}
}

// multiHyphen matches two or more consecutive hyphens, used by normalize to collapse them into one.
var multiHyphen = regexp.MustCompile(`-{2,}`)

// normalize converts a string to a Kubernetes-safe label value.
//
// Rules applied:
//  1. Lowercase
//  2. Spaces, underscores, and forward-slashes become hyphens
//  3. Characters outside [a-z0-9-.] are dropped
//  4. Leading/trailing hyphens and dots are stripped
//  5. Consecutive hyphens are collapsed to one
//  6. Result is truncated to 63 characters (K8s label value max)
func normalize(str string) string {
	str = strings.ToLower(strings.TrimSpace(str))

	var buffer strings.Builder
	buffer.Grow(len(str))

	for _, char := range str {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			buffer.WriteRune(char)
		case char == ' ', char == '_', char == '/':
			buffer.WriteRune('-')
		case char == '-', char == '.':
			buffer.WriteRune(char)
		}
	}

	result := strings.Trim(buffer.String(), "-.")
	result = multiHyphen.ReplaceAllString(result, "-")

	if len(result) > 63 {
		result = strings.TrimRight(result[:63], "-.")
	}

	return result
}
