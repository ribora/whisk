package validate

import (
	"fmt"
	"strings"
)

func IsStartswith(value string, args ...string) (bool, error) {
	if len(args) == 0 {
		return false, fmt.Errorf("startswith: argument required")
	}
	return strings.HasPrefix(value, args[0]), nil
}
