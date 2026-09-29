package validate

import (
	"fmt"
	"sort"
	"strings"
)

// Sorted by key: the output is compared against a golden.
func fieldSegmentCode(f field, extractTags []string) string {
	scanTag := strings.TrimSpace(f.Tag.Get("scan"))
	if f.Name == "" || scanTag == "embed" {
		return ""
	}
	pairs := []string{fmt.Sprintf("%q, %q", "go", f.Name)}
	names := map[string]string{}
	for _, key := range extractTags {
		if key == "go" {
			continue
		}
		v := strings.Split(f.Tag.Get(key), ",")[0]
		if v == "" || v == "-" {
			v = scanKey(scanTag)
		}
		if v != "" {
			names[key] = v
		}
	}
	keys := make([]string, 0, len(names))
	for k := range names {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%q, %q", k, names[k]))
	}
	return "validate.NewFieldSegment(" + strings.Join(pairs, ", ") + ")"
}

func scanKey(scanTag string) string {
	_, rest, ok := strings.Cut(scanTag, "=")
	if !ok {
		return ""
	}
	key, _, _ := strings.Cut(rest, ",")
	return strings.TrimSpace(key)
}

// "now", a duration such as "7d" or "-1h", or an ISO date — the forms
// validate.IsTimeMin and friends accept.
func isTimeValue(v string) bool {
	if v == "now" {
		return true
	}
	if strings.HasSuffix(v, "d") || strings.HasSuffix(v, "days") ||
		strings.HasSuffix(v, "h") || strings.HasSuffix(v, "m") {
		if len(v) > 1 && (isDigit(v[0]) || v[0] == '-') {
			return true
		}
	}
	if len(v) == 10 && v[4] == '-' && v[7] == '-' {
		return true
	}
	return false
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
