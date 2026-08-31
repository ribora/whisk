package named_scalars

import (
	"context"
	"errors"
	"testing"

	"gitlab.com/gorib/whisk/scan"
)

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

func TestNamedTypesConvert(t *testing.T) {
	f, err := scan.Into[Filter](values{
		"query/status":   {"active"},
		"query/page":     {"3"},
		"query/ratio":    {"0.25"},
		"query/verbose":  {"true"},
		"query/statuses": {"active, archived ,"},
		"query/pages":    {"1", "2"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if f.Status != Status("active") {
		t.Errorf("Status = %q", f.Status)
	}
	if f.Page != Page(3) {
		t.Errorf("Page = %d", f.Page)
	}
	if f.Ratio != Ratio(0.25) {
		t.Errorf("Ratio = %v", f.Ratio)
	}
	if f.Verbose != Verbose(true) {
		t.Errorf("Verbose = %v", f.Verbose)
	}
	// Trailing comma and the padded element: trimmed, and the empty part skipped.
	want := []Status{"active", "archived"}
	if len(f.Statuses) != len(want) {
		t.Fatalf("Statuses = %v, want %v", f.Statuses, want)
	}
	for i := range want {
		if f.Statuses[i] != want[i] {
			t.Errorf("Statuses[%d] = %q, want %q", i, f.Statuses[i], want[i])
		}
	}
	if len(f.Pages) != 2 || f.Pages[0] != Page(1) || f.Pages[1] != Page(2) {
		t.Errorf("Pages = %v, want [1 2]", f.Pages)
	}
}

func TestBadNumberIsPayloadError(t *testing.T) {
	_, err := scan.Into[Filter](values{"query/page": {"three"}})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, scan.ErrPayload) {
		t.Errorf("error %v does not wrap scan.ErrPayload", err)
	}
}
