package validate

import (
	"fmt"
	"strings"
)

func IsContains(value string, args ...string) (bool, error) {
	if len(args) == 0 {
		return false, fmt.Errorf("contains: argument required")
	}
	return strings.Contains(value, args[0]), nil
}
