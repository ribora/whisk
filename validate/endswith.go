package validate

import (
	"fmt"
	"strings"
)

func IsEndswith(value string, args ...string) (bool, error) {
	if len(args) == 0 {
		return false, fmt.Errorf("endswith: argument required")
	}
	return strings.HasSuffix(value, args[0]), nil
}
