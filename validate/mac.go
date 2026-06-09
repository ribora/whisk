package validate

import "net"

func IsMac(value string, args ...string) (bool, error) {
	_, err := net.ParseMAC(value)
	if err != nil {
		return false, err
	}
	return true, nil
}
