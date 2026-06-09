package validate

import "time"

func IsTimezone(value string, args ...string) (bool, error) {
	// "Local" is host-specific — reject so config doesn't silently drift between envs.
	if value == "" || value == "Local" {
		return false, nil
	}
	_, err := time.LoadLocation(value)
	if err != nil {
		return false, err
	}
	return true, nil
}
