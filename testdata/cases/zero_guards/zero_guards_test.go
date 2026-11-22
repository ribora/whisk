package zero_guards

import (
	"errors"
	"strings"
	"testing"

	"github.com/ribora/whisk/validate"
)

// A zero Doc is the absent-everything case. Only required may speak, and nothing
// may panic: the *ID guard is what keeps the generated code from dereferencing a
// nil pointer. Explicit is a struct value, which is present whether or not it was
// filled, so its own required fires too.
func TestZeroValueOnlyFailsRequired(t *testing.T) {
	want := map[string]string{
		"id_req":        "required",
		"page_req":      "required",
		"when_req":      "required",
		"explicit.name": "required",
	}

	err := (&Doc{}).Validate()
	if err == nil {
		t.Fatal("expected the required fields to fail")
	}

	seen := map[string]bool{}
	for _, e := range unwrap(t, err) {
		var ve *validate.ValidationError
		if !errors.As(e, &ve) {
			t.Fatalf("unexpected error %v", e)
		}
		field := ve.Segments.Format(validate.JSONFormatter)
		tag, expected := want[field]
		if !expected {
			t.Errorf("%s spoke up on a zero value with %q", field, ve.Tag)
			continue
		}
		if ve.Tag != tag {
			t.Errorf("%s reported %q, want %q", field, ve.Tag, tag)
		}
		seen[field] = true
	}
	for field := range want {
		if !seen[field] {
			t.Errorf("%s stayed silent, want %q", field, want[field])
		}
	}
}

// The kind reaches the client, so a named scalar has to report what it is rather
// than the "obj" its declared name once earned it.
func TestKindFollowsTheUnderlyingType(t *testing.T) {
	want := map[string]string{
		"id_req":   validate.KindStr,
		"page_req": validate.KindValue,
		"when_req": validate.KindTime,
	}
	for _, e := range unwrap(t, (&Doc{}).Validate()) {
		var ve *validate.ValidationError
		if !errors.As(e, &ve) {
			continue
		}
		field := ve.Segments.Format(validate.JSONFormatter)
		if got, ok := want[field]; ok && ve.Kind != got {
			t.Errorf("%s: Kind = %q, want %q", field, ve.Kind, got)
		}
	}
}

// Filled in, the named fields are checked like the plain ones.
func TestFilledNamedFieldsAreChecked(t *testing.T) {
	bad := ID("not-a-uuid")
	err := (&Doc{ID: "also-not", Ptr: &bad, Slug: "ab", IDs: []ID{"x", "x"}}).Validate()
	if err == nil {
		t.Fatal("expected failures")
	}
	msg := err.Error()
	for _, want := range []string{"'ID'", "'Ptr'", "'Slug'", "'IDs'"} {
		if !strings.Contains(msg, want) {
			t.Errorf("%s missing from %q", want, msg)
		}
	}
	// Page stays absent, so its gte does not fire.
	if strings.Contains(msg, "'Page' failed") {
		t.Errorf("Page fired on an absent optional field: %q", msg)
	}
}

// An element follows the same rule a field does: an empty one is absent, and
// only required speaks about absence. So dive,uuid lets it through and
// dive,required,uuid refuses it — with one complaint, not two.
func TestEmptyElementNeedsRequired(t *testing.T) {
	// The other required fields of a zero Doc always fire, so look only at the
	// element paths.
	if got := tagsAt(t, &Doc{IDs: []ID{""}}, "ids[0]"); len(got) != 0 {
		t.Errorf("dive,uuid should let an empty element pass, got %v", got)
	}
	got := tagsAt(t, &Doc{Strict: []ID{""}}, "strict[0]")
	if len(got) != 1 || got[0] != "required" {
		t.Errorf("strict[0] reported %v, want just [required]", got)
	}
}

func tagsAt(t *testing.T, doc *Doc, path string) []string {
	t.Helper()
	err := doc.Validate()
	if err == nil {
		return nil
	}
	var out []string
	for _, e := range unwrap(t, err) {
		var ve *validate.ValidationError
		if !errors.As(e, &ve) {
			continue
		}
		if ve.Segments.Format(validate.JSONFormatter) == path {
			out = append(out, ve.Tag)
		}
	}
	return out
}

func unwrap(t *testing.T, err error) []error {
	t.Helper()
	me, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return []error{err}
	}
	return me.Unwrap()
}
