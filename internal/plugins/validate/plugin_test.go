package validate_test

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ribora/whisk/internal/generator"
	"github.com/ribora/whisk/internal/plugins/validate"
	"github.com/ribora/whisk/internal/shared"
	"github.com/ribora/whisk/internal/testmodule"
)

func runValidate(t *testing.T, source string, opts validate.Options) generator.Section {
	t.Helper()
	dir := testmodule.New(t, source)
	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatalf("shared.Load: %v", err)
	}
	sec, err := validate.New(opts).Generate(structs)
	if err != nil {
		t.Fatalf("validate.Generate: %v", err)
	}
	return sec
}

func runValidateExpectError(t *testing.T, source, want string) {
	t.Helper()
	dir := testmodule.New(t, source)
	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatalf("shared.Load: %v", err)
	}
	_, err = validate.New(validate.Options{}).Generate(structs)
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %v", want, err)
	}
}

func TestGenerate_skipsStructsWithoutContributors(t *testing.T) {
	sec := runValidate(t, `package sample
type Plain struct {
	Name string
	Age  int
}
`, validate.Options{})
	if len(sec.Code) != 0 {
		t.Fatalf("expected empty section, got:\n%s", sec.Code)
	}
}

func TestGenerate_requiredString(t *testing.T) {
	sec := runValidate(t, `package sample
type R struct {
	Name string `+"`validate:\"required\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"func (s *R) Validate() error",
		`if s.Name == "" {`,
		`Tag:  "required"`,
		`Kind: validate.KindStr`,
		`validate.NewFieldSegment("go", "Name")`,
	)
}

func TestGenerate_comparisonStringLen(t *testing.T) {
	sec := runValidate(t, `package sample
type R struct {
	Password string `+"`validate:\"required,min=8,max=64\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"utf8.RuneCountInString(s.Password) < 8",
		"utf8.RuneCountInString(s.Password) > 64",
		`Tag:   "min"`,
		`Tag:   "max"`,
	)
	if !contains(importPaths(sec.Imports), "unicode/utf8") {
		t.Errorf("imports missing unicode/utf8: %v", sec.Imports)
	}
}

func TestGenerate_timeComparison(t *testing.T) {
	sec := runValidate(t, `package sample
import "time"
type R struct {
	When time.Time `+"`validate:\"gt=2020-01-01\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code, `validate.IsTimeGT(s.When, "2020-01-01")`)
}

func TestGenerate_diveSlicePrimitive(t *testing.T) {
	sec := runValidate(t, `package sample
type R struct {
	Codes []string `+"`validate:\"dive,min=2\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"for i, v := range s.Codes",
		"utf8.RuneCountInString(v) < 2",
		"validate.NewIndexSegment(i)",
	)
}

func TestGenerate_diveSliceNamedStruct(t *testing.T) {
	sec := runValidate(t, `package sample
type Item struct {
	Name string `+"`validate:\"required\"`"+`
}
type R struct {
	Items []Item
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"for i, v := range s.Items",
		"if subErr := v.Validate(); subErr != nil",
		"validate.AppendSubErrors(&errs, subErr",
		"validate.NewIndexSegment(i)",
	)
	if strings.Contains(code, "interface{ Validate() error }") {
		t.Errorf("expected a direct call, not a type assertion:\n%s", code)
	}
}

func TestGenerate_autoAddDive(t *testing.T) {
	// Inner has validation, Outer references it without explicit ,dive.
	// The auto-dive should add it implicitly.
	sec := runValidate(t, `package sample
type Inner struct {
	X int `+"`validate:\"min=1\"`"+`
}
type Outer struct {
	I Inner
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"func (s *Outer) Validate()",
		"if subErr := s.I.Validate(); subErr != nil",
	)
}

func TestGenerate_unique(t *testing.T) {
	sec := runValidate(t, `package sample
type R struct {
	IDs []string `+"`validate:\"unique\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"seenIDs := make(map[string]bool, len(s.IDs))",
		"for _, v := range s.IDs",
		"if seenIDs[v]",
		`Tag:  "unique"`,
	)
}

func TestGenerate_uniqueByField(t *testing.T) {
	sec := runValidate(t, `package sample
type Item struct{ Name string }
type R struct {
	Items []Item `+"`validate:\"unique=Name\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"seenItemsName := make(map[any]bool",
		"for i, v := range s.Items",
		"if seenItemsName[v.Name]",
		`Value: "Name"`,
	)
}

func TestGenerate_uniqueByFieldOnAnonStruct_missingField(t *testing.T) {
	runValidateExpectError(t, `package sample
type R struct {
	Items []struct{ Other string } `+"`validate:\"unique=Name\"`"+`
}
`, "has no field")
}

func TestGenerate_builtInValidator(t *testing.T) {
	sec := runValidate(t, `package sample
type R struct {
	Email string `+"`validate:\"required,email\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code, "validate.IsEmail(s.Email)")
}

func TestGenerate_builtInValidatorReturningError(t *testing.T) {
	sec := runValidate(t, `package sample
type R struct {
	ID string `+"`validate:\"uuid\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"validate.IsUuid(s.ID)",
		"Err:   errCapture",
	)
}

func TestGenerate_embeddedStructValidate(t *testing.T) {
	sec := runValidate(t, `package sample
type Base struct {
	Name string `+"`validate:\"required\"`"+`
}
type R struct {
	Base
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"func (s *R) Validate()",
		"err := s.Base.Validate()",
		"validate.AppendSubErrors(&errs, err)",
	)
}

func TestGenerate_embedWithoutValidateIsSkipped(t *testing.T) {
	// Plain has no validation, so embedding it must NOT emit s.Plain.Validate()
	// (Plain gets no generated Validate). User's own tag keeps User contributing.
	sec := runValidate(t, `package sample
type Plain struct{ X int }
type User struct {
	Plain
	Name string `+"`validate:\"required\"`"+`
}
`, validate.Options{})
	code := string(sec.Code)
	must(t, code, "func (s *User) Validate()", `if s.Name == ""`)
	if strings.Contains(code, "s.Plain.Validate()") {
		t.Errorf("must not delegate to a validation-less embed:\n%s", code)
	}
	if strings.Contains(code, "func (s *Plain) Validate()") {
		t.Errorf("Plain has no validation and should get no Validate:\n%s", code)
	}
}

func TestGenerate_tagsOnGrandchildDirectCall(t *testing.T) {
	// Middle has no own tags; only its named field Inner does. The chain must
	// still resolve — every level gets a Validate and dives by direct call.
	sec := runValidate(t, `package sample
type Inner struct { Email string `+"`validate:\"required\"`"+` }
type Middle struct { Inner Inner }
type Outer struct { Middle Middle }
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"func (s *Inner) Validate()",
		"func (s *Middle) Validate()",
		"func (s *Outer) Validate()",
		"if subErr := s.Middle.Validate(); subErr != nil",
		"if subErr := s.Inner.Validate(); subErr != nil",
	)
	if strings.Contains(code, "interface{ Validate() error }") {
		t.Errorf("concrete grandchildren should use direct calls:\n%s", code)
	}
}

func TestGenerate_noValidationAnywhereSkipsStruct(t *testing.T) {
	// Nothing has validate tags → no Validate methods at all (no empty bloat).
	sec := runValidate(t, `package sample
type Inner struct{ X int }
type Outer struct{ Inner Inner }
`, validate.Options{})
	if len(sec.Code) != 0 {
		t.Errorf("expected no output, got:\n%s", sec.Code)
	}
}

func TestGenerate_validateHook(t *testing.T) {
	sec := runValidate(t, `package sample
type R struct {
	X int `+"`validate:\"min=1\"`"+`
}
func (s *R) validate() error { return nil }
`, validate.Options{})
	code := string(sec.Code)
	must(t, code,
		"e := s.validate()",
		"validate.AppendSubErrors(&errs, e)",
	)
}

func TestGenerate_emittedCodeParsesAsGo(t *testing.T) {
	sec := runValidate(t, `package sample
import "time"
type Item struct{ Name string `+"`validate:\"required\"`"+` }
type R struct {
	Required string `+"`validate:\"required,min=4,max=32\"`"+`
	Email    string `+"`validate:\"required,email\"`"+`
	When     time.Time `+"`validate:\"gte=2020-01-01\"`"+`
	IDs      []string `+"`validate:\"unique\"`"+`
	Items    []Item
	Optional *string `+"`validate:\"required\"`"+`
}
`, validate.Options{ExtractTags: []string{"json"}})

	header := "package sample\n\nimport (\n"
	for _, imp := range importPaths(sec.Imports) {
		if strings.Contains(imp, " ") {
			parts := strings.SplitN(imp, " ", 2)
			header += "\t" + parts[0] + " \"" + parts[1] + "\"\n"
		} else {
			header += "\t\"" + imp + "\"\n"
		}
	}
	header += ")\n\nimport _ \"time\"\n\n"
	src := header + string(sec.Code)
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("emitted code doesn't parse:\n%s\nerr: %v", src, err)
	}
}

func TestGenerate_extractTagsInSegments(t *testing.T) {
	sec := runValidate(t, `package sample
type R struct {
	Name string `+"`validate:\"required\" json:\"display_name\"`"+`
}
`, validate.Options{ExtractTags: []string{"json"}})
	code := string(sec.Code)
	must(t, code, `validate.NewFieldSegment("go", "Name", "json", "display_name")`)
}

func TestGenerate_requiredOnBoolError(t *testing.T) {
	runValidateExpectError(t, `package sample
type R struct {
	Active bool `+"`validate:\"required\"`"+`
}
`, "required tag doesn't make sense for bool")
}

// Named field, so the author does not have to grep for the tag.
func TestGenerate_unknownTag(t *testing.T) {
	runValidateExpectError(t, `package sample
type R struct {
	X string `+"`validate:\"required,bogusvalidator\"`"+`
}
`, `R.X: unknown validator tag "bogusvalidator"`)
}

// The tag maps to a func name, not to a file name.
func TestGenerate_customValidatorInAnyFile(t *testing.T) {
	storage := t.TempDir()
	writeChecks(t, storage, "rules.go", "func IsPasscode(v string, args ...string) bool { return true }")

	sec := runValidate(t, `package sample
type R struct {
	Code string `+"`validate:\"passcode\"`"+`
}
`, validate.Options{StoragePath: storage, StorageLabel: "pkg/validate", UserChecksImport: "mod/pkg/validate"})

	must(t, string(sec.Code), "checks.IsPasscode(s.Code)")
	if len(sec.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", sec.Warnings)
	}
}

// The one case where the model's reader cannot tell which check runs.
func TestGenerate_overridingBuiltInWarns(t *testing.T) {
	storage := t.TempDir()
	writeChecks(t, storage, "rules.go", "func IsEmail(v string, args ...string) bool { return true }")

	sec := runValidate(t, `package sample
type R struct {
	Mail string `+"`validate:\"email\"`"+`
}
`, validate.Options{StoragePath: storage, StorageLabel: "pkg/validate", UserChecksImport: "mod/pkg/validate"})

	must(t, string(sec.Code), "checks.IsEmail(s.Mail)")
	if len(sec.Warnings) != 1 || !strings.Contains(sec.Warnings[0], `overrides the built-in "email"`) {
		t.Errorf("warnings = %v, want one about overriding email", sec.Warnings)
	}
}

// A near miss (IsPassCode) is the case the search note exists for.
func TestGenerate_unknownTagNamesTheSearch(t *testing.T) {
	storage := t.TempDir()
	writeChecks(t, storage, "rules.go", "func IsPassCode(v string) bool { return true }")

	dir := testmodule.New(t, `package sample
type R struct {
	Code string `+"`validate:\"passcode\"`"+`
}
`)
	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatalf("shared.Load: %v", err)
	}
	_, err = validate.New(validate.Options{StoragePath: storage, StorageLabel: "pkg/validate"}).Generate(structs)
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"IsPasscode", "pkg/validate", "found there: IsPassCode"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// ---- helpers ----

func writeChecks(t *testing.T, dir, file, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte("package checks\n\n"+body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func must(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			t.Errorf("missing %q in:\n%s", n, haystack)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func importPaths(imports []generator.Import) []string {
	out := make([]string, len(imports))
	for i, imp := range imports {
		out[i] = imp.Path
	}
	return out
}
