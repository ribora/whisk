package validate

import "strings"

// Requires at least one dot: "localhost" → false, "localhost.local" → true.
func IsFqdn(value string, args ...string) bool {
	if !strings.Contains(value, ".") {
		return false
	}
	return IsHostname(value)
}
