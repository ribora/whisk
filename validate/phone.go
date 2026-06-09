package validate

import "regexp"

var phoneE164RE = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// phone=e164 — E.164 international format: +12025550123
func IsPhone(value string, args ...string) bool {
	if len(args) > 0 && args[0] != "" && args[0] != "e164" {
		return false
	}
	return phoneE164RE.MatchString(value)
}
