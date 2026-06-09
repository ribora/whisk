package validate

import "net"

func IsCidr(value string, args ...string) (bool, error) {
	_, _, err := net.ParseCIDR(value)
	if err != nil {
		return false, err
	}
	return true, nil
}
