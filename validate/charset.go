package validate

import (
	"fmt"
	"unicode"
)

// charset=alpha|alphanum|numeric|hex — argument required.
func IsCharset(value string, args ...string) (bool, error) {
	if value == "" {
		return false, nil
	}
	if len(args) == 0 {
		return false, fmt.Errorf("charset: argument required (alpha|alphanum|numeric|hex)")
	}
	switch args[0] {
	case "alpha":
		for _, r := range value {
			if !unicode.IsLetter(r) {
				return false, nil
			}
		}
	case "alphanum":
		for _, r := range value {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return false, nil
			}
		}
	case "numeric":
		for _, r := range value {
			if !unicode.IsDigit(r) {
				return false, nil
			}
		}
	case "hex":
		for _, r := range value {
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false, nil
			}
		}
	default:
		return false, fmt.Errorf("charset: unknown set %q (want alpha|alphanum|numeric|hex)", args[0])
	}
	return true, nil
}
