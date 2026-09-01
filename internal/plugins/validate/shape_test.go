package validate_test

import (
	"strings"
	"testing"

	"gitlab.com/gorib/whisk/internal/plugins/validate"
)

// Both the guard and required's zero check used to be read off how a type is
// spelled, so a named scalar looked like a struct: no guard, no required at all,
// and a Kind of "obj" for a string. The classifier now answers from the
// underlying type.
func TestNamedScalarsAreScalars(t *testing.T) {
	sec := runValidate(t, `package sample

type ID string
type Page int

type R struct {
	ID      ID   `+"`validate:\"uuid\"`"+`
	IDReq   ID   `+"`validate:\"required,uuid\"`"+`
	Page    Page `+"`validate:\"gte=1\"`"+`
	PageReq Page `+"`validate:\"required\"`"+`
	Ptr     *ID  `+"`validate:\"uuid\"`"+`
	Len     ID   `+"`validate:\"min=3\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)

	must(t, code,
		// The optional fields stay quiet when empty.
		`if s.ID != "" {`,
		"if s.Page != 0 {",
		// required is emitted at all, which it was not.
		`if s.IDReq == "" {`,
		"if s.PageReq == 0 {",
		// A pointer is checked before it is dereferenced.
		"if s.Ptr != nil {",
		// The kind that reaches the client names the underlying type.
		"Kind: validate.KindStr",
		"Kind: validate.KindValue",
		// A named string is converted for the calls that take one exactly.
		"validate.IsUuid(string(s.ID))",
		"utf8.RuneCountInString(string(s.Len))",
	)
	if strings.Contains(code, "KindObj") {
		t.Errorf("a named scalar reported obj:\n%s", code)
	}
	if strings.Contains(code, "*s.Ptr)") && !strings.Contains(code, "string(*s.Ptr)") {
		t.Errorf("the dereference should be converted:\n%s", code)
	}
}

// Both of these once emitted `if x != 0` around a value that is not a number,
// which does not compile. The golden case compiles; this says why it would fail.
func TestNonNumericFieldsAreNotComparedToZero(t *testing.T) {
	sec := runValidate(t, `package sample

import "time"

type Meta struct {
	Name string `+"`validate:\"required\"`"+`
}

type R struct {
	Explicit Meta      `+"`validate:\"dive\"`"+`
	When     time.Time `+"`validate:\"gt=2020-01-01\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)

	if strings.Contains(code, "!= 0") {
		t.Errorf("a struct or a time was compared to 0:\n%s", code)
	}
	must(t, code,
		"if !s.When.IsZero() {",
		"if subErr := s.Explicit.Validate(); subErr != nil {",
	)
}

// An element carries its rules the way a field does, guard included, so the one
// rule holds everywhere: emptiness is required's business. Which is also what
// keeps an empty element from reporting required and min at once.
func TestElementsAreGuardedLikeFields(t *testing.T) {
	sec := runValidate(t, `package sample
type R struct {
	Loose  []string `+"`validate:\"dive,min=2\"`"+`
	Strict []string `+"`validate:\"dive,required,min=2\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)

	must(t, code,
		`if v != "" {`,
		"utf8.RuneCountInString(v) < 2",
		// required is what refuses an empty element, and it sits outside the guard.
		`if v == "" {`,
	)
	if strings.Count(code, `if v != ""`) != 2 {
		t.Errorf("both element loops should carry the guard:\n%s", code)
	}
}

// A named element reaches its rules too — they used to be dropped in silence,
// because the element was neither a basic type by name nor a struct to dive into.
func TestNamedElementsReachTheirRules(t *testing.T) {
	sec := runValidate(t, `package sample
type ID string
type R struct {
	IDs []ID `+"`validate:\"dive,uuid\"`"+`
}
`, validate.Options{})
	must(t, string(sec.Code), "validate.IsUuid(string(v))")
}
