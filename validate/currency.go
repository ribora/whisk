package validate

import "golang.org/x/text/currency"

func IsCurrency(value string, args ...string) (bool, error) {
	_, err := currency.ParseISO(value)
	if err != nil {
		return false, err
	}
	return true, nil
}
