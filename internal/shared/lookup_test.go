package shared_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ribora/whisk/internal/shared"
)

func TestPascalCase(t *testing.T) {
	cases := map[string]string{
		"":            "",
		"passcode":    "Passcode",
		"company_id":  "CompanyId",
		"a_b_c":       "ABC",
		"_leading":    "Leading",
		"trailing_":   "Trailing",
		"double__sep": "DoubleSep",
		"Already":     "Already",
	}
	for in, want := range cases {
		if got := shared.PascalCase(in); got != want {
			t.Errorf("PascalCase(%q) = %q, want %q", in, got, want)
		}
	}
}

// This lookup is the whole of "a rule is a func, not a registration", so what it
// does and does not accept is the contract.
func TestIndexFuncs(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.go", "package checks\n\nfunc IsPasscode(v string, args ...string) bool { return true }\n")
	write(t, dir, "method.go", "package checks\n\ntype Rules struct{}\n\nfunc (Rules) IsMethod(v string) bool { return true }\n")
	write(t, dir, "helper_test.go", "package checks\n\nfunc IsTestOnly(v string) bool { return true }\n")

	idx, err := shared.IndexFuncs(dir)
	if err != nil {
		t.Fatalf("IndexFuncs: %v", err)
	}
	// rules.go, not passcode.go: only the func name carries the tag.
	if idx["IsPasscode"] == nil {
		t.Error("a package-level func was not found outside the file named after the tag")
	}
	// checks.IsMethod is not callable, so matching it would emit a call to nothing.
	if idx["IsMethod"] != nil {
		t.Error("a method was accepted as a package-level func")
	}
	// Not part of the package the generated code imports.
	if idx["IsTestOnly"] != nil {
		t.Error("a func from a _test.go file was indexed")
	}
	if idx["IsOther"] != nil {
		t.Error("a name that is not there was found")
	}
}

// A project with no custom rules looks exactly like this, so the caller — not
// the lookup — decides whether anything was missing.
func TestIndexFuncs_absentIsEmpty(t *testing.T) {
	for _, dir := range []string{"", filepath.Join(t.TempDir(), "absent")} {
		idx, err := shared.IndexFuncs(dir)
		if err != nil {
			t.Errorf("IndexFuncs(%q): %v", dir, err)
		}
		if len(idx) != 0 {
			t.Errorf("IndexFuncs(%q) = %v, want empty", dir, idx)
		}
	}
}

// A whole directory is parsed, so a silent miss would report every tag in the
// project as unknown.
func TestIndexFuncs_unparseableFileErrors(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "broken.go", "package checks\n\nfunc IsBroken(")

	if _, err := shared.IndexFuncs(dir); err == nil {
		t.Fatal("an unparseable file was reported as an empty index")
	} else if !strings.Contains(err.Error(), "broken.go") {
		t.Errorf("error %q does not name the file", err)
	}
}

func TestIndexFuncs_fileInsteadOfDir(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "checks.go", "package checks\n")

	if _, err := shared.IndexFuncs(filepath.Join(dir, "checks.go")); err == nil ||
		!strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("expected a not-a-directory error, got %v", err)
	}
}

// This wording is what keeps a failed lookup from reading as magic, so it is
// asserted rather than left to drift.
func TestSearchNote(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "rules.go", "package checks\n\nfunc IsPassCode(v string) bool { return true }\n")
	idx, err := shared.IndexFuncs(dir)
	if err != nil {
		t.Fatal(err)
	}

	if got := shared.SearchNote(dir, "Is", idx); got != "(found there: IsPassCode)" {
		t.Errorf("got %q, want the candidate listed", got)
	}
	if got := shared.SearchNote(filepath.Join(dir, "absent"), "Is", nil); got != "(directory does not exist)" {
		t.Errorf("got %q, want the missing directory reported", got)
	}
	if got := shared.SearchNote(t.TempDir(), "Is", nil); got != "(no Is* funcs there)" {
		t.Errorf("got %q, want the empty directory reported", got)
	}
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
