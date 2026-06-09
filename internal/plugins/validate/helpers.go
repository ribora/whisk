package validate

import (
	"fmt"
	"sort"
	"strings"
)

// fieldSegmentCode builds the `validate.NewFieldSegment(...)` call expression
// for a field, listing its Go name and any extra tag-based aliases (json,
// xml, etc.). Names are sorted by key so output is deterministic.
func fieldSegmentCode(f field, extractTags []string) string {
	if f.Name == "" {
		return ""
	}
	pairs := []string{fmt.Sprintf("%q, %q", "go", f.Name)}
	names := map[string]string{}
	for _, key := range extractTags {
		if key == "go" {
			continue
		}
		if v := strings.Split(f.Tag.Get(key), ",")[0]; v != "" && v != "-" {
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

// isTimeValue reports whether v looks like a time literal accepted by
// validate.IsTimeMin / IsTimeMax / IsTimeGT / IsTimeLT.
//
// Recognised forms:
//   - "now"
//   - duration suffixes: "d", "days", "h", "m" (e.g. "7d", "-1h", "30days")
//   - ISO date "YYYY-MM-DD"
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
