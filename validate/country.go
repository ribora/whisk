package validate

import "golang.org/x/text/language"

func IsCountry(value string, args ...string) (bool, error) {
	if len(value) != 2 {
		return false, nil
	}
	r, err := language.ParseRegion(value)
	if err != nil {
		return false, err
	}
	// ParseRegion accepts numeric and alpha-3 too — require canonical alpha-2.
	return r.String() == value, nil
}
