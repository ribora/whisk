package validate

import (
	"strings"

	"golang.org/x/text/language"
)

func IsCountry(value string, args ...string) (bool, error) {
	if len(value) != 2 {
		return false, nil
	}
	r, err := language.ParseRegion(value)
	if err != nil {
		return false, err
	}
	// ParseRegion accepts numeric and alpha-3 too, which the length check above
	// already turned away. What is left is case, and that is the client's to
	// choose — currency folds it as well.
	return strings.EqualFold(r.String(), value), nil
}
