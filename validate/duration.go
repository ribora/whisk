package validate

import "time"

func IsDuration(value string, args ...string) (bool, error) {
	_, err := time.ParseDuration(value)
	if err != nil {
		return false, err
	}
	return true, nil
}
