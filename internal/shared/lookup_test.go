package shared_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/gorib/whisk/internal/shared"
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

// This lookup is the whole of "a rule is a file, not a registration", so what it
// does and does not accept is the contract.
func TestFindFunc(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("passcode.go", "package checks\n\nfunc IsPasscode(v string, args ...string) bool { return true }\n")
	write("method.go", "package checks\n\ntype Rules struct{}\n\nfunc (Rules) IsMethod(v string) bool { return true }\n")
	write("broken.go", "package checks\n\nfunc IsBroken(")

	if fn := shared.FindFunc(dir, "passcode.go", "IsPasscode"); fn == nil {
		t.Error("a package-level func was not found")
	}
	// A method is not callable as checks.IsMethod, so finding it would emit a call
	// to something that does not exist.
	if fn := shared.FindFunc(dir, "method.go", "IsMethod"); fn != nil {
		t.Error("a method was accepted as a package-level func")
	}
	if fn := shared.FindFunc(dir, "passcode.go", "IsOther"); fn != nil {
		t.Error("a name that is not there was found")
	}
	if fn := shared.FindFunc(dir, "absent.go", "IsPasscode"); fn != nil {
		t.Error("a missing file was found")
	}
	if fn := shared.FindFunc("", "passcode.go", "IsPasscode"); fn != nil {
		t.Error("an empty directory was searched")
	}
	// A file that does not parse is a silent miss rather than a crash: the tag then
	// reports as unsupported, which is the error the author can act on.
	if fn := shared.FindFunc(dir, "broken.go", "IsBroken"); fn != nil {
		t.Error("an unparseable file yielded a declaration")
	}
}
