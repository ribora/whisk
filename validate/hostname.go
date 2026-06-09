package validate

import "regexp"

var hostnameRE = regexp.MustCompile(`^(?i)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)

func IsHostname(value string, args ...string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	return hostnameRE.MatchString(value)
}
