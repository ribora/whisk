package narrow_scalars

import (
	"context"
	"errors"
	"testing"

	"gitlab.com/gorib/whisk/scan"
)

// values is a Source backed by a map, so a case can be exercised without any
// transport. The key is "scope/name" because scopes are opaque strings.
type values map[string][]string

func (v values) Lookup(scope, key string) string {
	if got := v[scope+"/"+key]; len(got) > 0 {
		return got[0]
	}
	return ""
}

func (v values) LookupAll(scope, key string) []string { return v[scope+"/"+key] }

func (v values) Body() []byte { return nil }

func (v values) Context() context.Context { return context.Background() }

func TestFitsInTheDeclaredWidth(t *testing.T) {
	got, err := scan.Into[Filter](values{
		"query/plain_small": {"127"},
		"query/small":       {"-128"},
		"query/plain_port":  {"65535"},
		"query/port":        {"8080"},
		"query/plain_ratio": {"1.5"},
		"query/ratio":       {"-2.25"},
		"query/smalls":      {"1", "2"},
		"query/count":       {"42"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := Filter{
		PlainSmall: 127,
		Small:      -128,
		PlainPort:  65535,
		Port:       8080,
		PlainRatio: 1.5,
		Ratio:      -2.25,
		Smalls:     []Small{1, 2},
		Count:      42,
	}
	if got.PlainSmall != want.PlainSmall || got.Small != want.Small ||
		got.PlainPort != want.PlainPort || got.Port != want.Port ||
		got.PlainRatio != want.PlainRatio || got.Ratio != want.Ratio ||
		got.Count != want.Count || len(got.Smalls) != 2 ||
		got.Smalls[0] != want.Smalls[0] || got.Smalls[1] != want.Smalls[1] {
		t.Errorf("Filter = %+v, want %+v", *got, want)
	}
}

// A value the field cannot hold is a parse error, not a wrapped number — and a
// named type has to refuse it exactly as the plain one does.
func TestRefusesWhatTheFieldCannotHold(t *testing.T) {
	cases := []struct {
		name  string
		scope string
		value string
	}{
		{"plain int8 over its range", "query/plain_small", "300"},
		{"named int8 over its range", "query/small", "300"},
		{"plain uint16 over its range", "query/plain_port", "70000"},
		{"named uint16 over its range", "query/port", "70000"},
		{"named uint16 below zero", "query/port", "-1"},
		{"plain float32 over its range", "query/plain_ratio", "1e300"},
		{"named float32 over its range", "query/ratio", "1e300"},
		{"named int8 element over its range", "query/smalls", "300"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := scan.Into[Filter](values{c.scope: {c.value}})
			if err == nil {
				t.Fatalf("%s = %s was accepted, want a parse error", c.scope, c.value)
			}
			if !errors.Is(err, scan.ErrPayload) {
				t.Errorf("err = %v, want it to carry scan.ErrPayload", err)
			}
		})
	}
}
