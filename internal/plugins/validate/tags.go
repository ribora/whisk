package validate

import "strings"

// Tag is one parsed entry from a `validate:"a,b=v,c"` string.
type Tag struct {
	Name     string
	Value    string
	HasValue bool
}

// parseTags splits a comma-delimited validate string into Tag entries.
// Empty entries are skipped. Trailing-comma input is tolerated.
func parseTags(raw string) []Tag {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]Tag, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		eq := strings.Index(p, "=")
		if eq < 0 {
			out = append(out, Tag{Name: p})
			continue
		}
		out = append(out, Tag{
			Name:     p[:eq],
			Value:    p[eq+1:],
			HasValue: true,
		})
	}
	return out
}
