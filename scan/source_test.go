package scan_test

import (
	"errors"
	"testing"

	"gitlab.com/gorib/whisk/scan"
)

type parseOnly struct {
	body string
}

func (p *parseOnly) Parse(src scan.Source) error {
	p.body = string(src.Body())
	return nil
}

type failsToParse struct{}

var errParse = errors.New("bad payload")

func (f *failsToParse) Parse(scan.Source) error { return errParse }

type validated struct {
	parsed    bool
	validated bool
	invalid   bool
}

var errInvalid = errors.New("invalid")

func (v *validated) Parse(scan.Source) error {
	v.parsed = true
	return nil
}

func (v *validated) Validate() error {
	v.validated = true
	if v.invalid {
		return errInvalid
	}
	return nil
}

func TestFromBytes(t *testing.T) {
	src := scan.FromBytes([]byte(`{"a":1}`))

	if got, want := string(src.Body()), `{"a":1}`; got != want {
		t.Errorf("Body() = %q, want %q", got, want)
	}
	// A body-only source has no scopes at all, so every lookup is a miss rather
	// than a panic — generated code calls these unconditionally.
	if got := src.Lookup("query", "page"); got != "" {
		t.Errorf("Lookup = %q, want empty", got)
	}
	if got := src.LookupAll("query", "tag"); got != nil {
		t.Errorf("LookupAll = %v, want nil", got)
	}
	if src.Context() == nil {
		t.Error("Context() = nil")
	}
}

func TestIntoParsesAndValidates(t *testing.T) {
	got, err := scan.Into[validated](scan.FromBytes(nil))
	if err != nil {
		t.Fatalf("Into: %v", err)
	}
	if !got.parsed || !got.validated {
		t.Errorf("parsed=%v validated=%v, want both", got.parsed, got.validated)
	}
}

// Validate is optional: a model with no rules only has Parse, and Into must not
// require the second method to exist.
func TestIntoWithoutValidator(t *testing.T) {
	got, err := scan.Into[parseOnly](scan.FromBytes([]byte("hello")))
	if err != nil {
		t.Fatalf("Into: %v", err)
	}
	if got.body != "hello" {
		t.Errorf("body = %q, want %q", got.body, "hello")
	}
}

func TestIntoReturnsTheValidationError(t *testing.T) {
	got, err := scan.Into[invalidModel](scan.FromBytes(nil))
	if !errors.Is(err, errInvalid) {
		t.Fatalf("err = %v, want %v", err, errInvalid)
	}
	if got != nil {
		t.Errorf("got = %v, want nil on a failed validation", got)
	}
}

type invalidModel struct{}

func (invalidModel) Parse(scan.Source) error { return nil }

func (invalidModel) Validate() error { return errInvalid }

// A parse failure stops before validation — otherwise a half-filled model would
// be reported field by field on top of "the payload is not readable".
func TestIntoStopsAtAParseError(t *testing.T) {
	got, err := scan.Into[failsToParse](scan.FromBytes(nil))
	if !errors.Is(err, errParse) {
		t.Fatalf("err = %v, want %v", err, errParse)
	}
	if got != nil {
		t.Errorf("got = %v, want nil on a failed parse", got)
	}
}

type refusesToBeValidated struct{}

func (refusesToBeValidated) Parse(scan.Source) error { return errParse }

func (refusesToBeValidated) Validate() error {
	panic("Validate must not run after Parse failed")
}

func TestIntoDoesNotValidateAfterAParseError(t *testing.T) {
	if _, err := scan.Into[refusesToBeValidated](scan.FromBytes(nil)); !errors.Is(err, errParse) {
		t.Fatalf("err = %v, want %v", err, errParse)
	}
}
