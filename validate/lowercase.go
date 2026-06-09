package validate

import "strings"

func IsLowercase(value string, args ...string) bool {
	return value != "" && value == strings.ToLower(value)
}
