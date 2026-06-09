package validate

import (
	"fmt"
	"net"
)

// ip, ip=v4, ip=v6.
func IsIp(value string, args ...string) (bool, error) {
	ip := net.ParseIP(value)
	if ip == nil {
		return false, nil
	}
	if len(args) == 0 {
		return true, nil
	}
	switch args[0] {
	case "v4":
		return ip.To4() != nil, nil
	case "v6":
		return ip.To4() == nil && ip.To16() != nil, nil
	default:
		return false, fmt.Errorf("ip: unknown version %q (want v4|v6)", args[0])
	}
}
