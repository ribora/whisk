package validate

import (
	"fmt"
	"net/netip"
)

// ip, ip=v4, ip=v6.
func IsIp(value string, args ...string) (bool, error) {
	addr, err := netip.ParseAddr(value)
	// A scope makes the address host-specific, which is not what an API field
	// holds. net.ParseIP refused these too.
	if err != nil || addr.Zone() != "" {
		return false, nil
	}
	if len(args) == 0 {
		return true, nil
	}
	switch args[0] {
	case "v4":
		return addr.Is4(), nil
	case "v6":
		// Is6 counts the IPv4-mapped form as v6 and Is4 does not, which is the
		// way round the text reads. net.IP.To4 answered the opposite.
		return addr.Is6(), nil
	default:
		return false, fmt.Errorf("ip: unknown version %q (want v4|v6)", args[0])
	}
}
