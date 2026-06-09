package validate

import "regexp"

// #rgb, #rgba, #rrggbb, #rrggbbaa.
var hexcolorRE = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

func IsHexcolor(value string, args ...string) bool {
	return hexcolorRE.MatchString(value)
}
