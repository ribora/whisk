package validate

import "strings"

func IsUppercase(value string, args ...string) bool {
	return value != "" && value == strings.ToUpper(value)
}
