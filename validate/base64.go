package validate

import "encoding/base64"

func IsBase64(value string, args ...string) (bool, error) {
	_, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return false, err
	}
	return true, nil
}
