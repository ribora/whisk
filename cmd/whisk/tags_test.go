package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ribora/whisk/internal/plugins/scan"
	"github.com/ribora/whisk/internal/plugins/validate"
)

// The listing exists because the answer is per-project, so what it has to get
// right is the project half: which directory it read and what it found there.
func TestTagReport_projectValidators(t *testing.T) {
	root := t.TempDir()
	writeChecks(t, root, "pkg/validate", `package validate

func IsPasscode(value string, args ...string) bool { return len(value) == 6 }
func IsCompanyId(value string, args ...string) bool { return value != "" }
`)

	out := report(t, root, projectConfig{}, nil)
	for _, want := range []string{
		"passcode         IsPasscode",
		// Two words in the func name is two words in the tag, or the generator
		// would not find it either.
		"company_id       IsCompanyId",
		"from pkg/validate (the fallback for validate.storage)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not carry %q:\n%s", want, out)
		}
	}
}

// A custom check that replaces a built-in is the one case the generated code
// cannot be read to tell which check runs, so the listing has to say it.
func TestTagReport_shadowIsNamed(t *testing.T) {
	root := t.TempDir()
	writeChecks(t, root, "pkg/validate", `package validate

func IsCurrency(value string, args ...string) bool { return value == "RUB" }
`)

	out := report(t, root, projectConfig{}, nil)
	if !strings.Contains(out, "replacing whisk's built-in") {
		t.Errorf("a shadowed built-in is not called out:\n%s", out)
	}
	// Once, under the origin that answers it — listing it as whisk's own too
	// would claim the built-in still runs.
	if n := strings.Count(out, "IsCurrency"); n != 1 {
		t.Errorf("IsCurrency appears %d times, want 1:\n%s", n, out)
	}
}

// A name the convention cannot produce is dead weight in that directory: the
// generator will never resolve a tag to it, so saying nothing would leave the
// author believing it is wired. The realistic version is a lowercase second
// word — `Ispasscode` for what should have been `IsPasscode`.
func TestTagReport_unreachableFuncIsFlagged(t *testing.T) {
	root := t.TempDir()
	writeChecks(t, root, "pkg/validate", `package validate

func Ispasscode(value string, args ...string) bool { return true }
`)

	out := report(t, root, projectConfig{}, nil)
	if !strings.Contains(out, "no tag reaches this function") {
		t.Errorf("Ispasscode is not flagged as unreachable:\n%s", out)
	}
}

// The inverse of "Is" + PascalCase is not ToLower, and getting that wrong would
// print a tag the generator cannot resolve — worse than printing nothing, since
// an agent would copy it and then have to read a generation error.
func TestTagReport_twoWordNameKeepsItsUnderscore(t *testing.T) {
	root := t.TempDir()
	writeChecks(t, root, "pkg/validate", `package validate

func IsCompanyId(value string, args ...string) bool { return true }
`)

	out := report(t, root, projectConfig{}, nil)
	if strings.Contains(out, "companyid") {
		t.Errorf("the tag was derived by lowercasing, and that tag resolves to nothing:\n%s", out)
	}
	if !strings.Contains(out, "company_id") {
		t.Errorf("the report does not name the tag that actually reaches IsCompanyId:\n%s", out)
	}
}

// Same precedence as generation, or the listing would send the author to the
// wrong directory — which is the whole thing it is for.
func TestTagReport_namesWhichSourceWon(t *testing.T) {
	root := t.TempDir()
	writeChecks(t, root, "internal/checks", `package checks

func IsPasscode(value string, args ...string) bool { return true }
`)

	fromFile := projectConfig{Validate: validate.Config{StorageDir: "internal/checks"}}
	out := report(t, root, fromFile, nil)
	if !strings.Contains(out, "from internal/checks (validate.storage in whisk.yaml)") {
		t.Errorf("the file's directory is not reported as the source:\n%s", out)
	}
	if !strings.Contains(out, "passcode         IsPasscode") {
		t.Errorf("the report did not read the configured directory:\n%s", out)
	}

	// A flag beats the file, and the report says so rather than naming the file.
	flagged := validate.Config{StorageDir: "internal/checks"}
	out = reportWith(t, root, projectConfig{Validate: validate.Config{StorageDir: "somewhere/else"}},
		map[string]bool{"validate.storage": true}, scan.DefaultConfig(), flagged)
	if !strings.Contains(out, "(-validate.storage on this command line)") {
		t.Errorf("a flag is not reported as the source:\n%s", out)
	}
}

// An unset project is the common case and must not look broken.
func TestTagReport_emptyProject(t *testing.T) {
	out := report(t, t.TempDir(), projectConfig{}, nil)
	if !strings.Contains(out, "nothing there yet") {
		t.Errorf("an empty project does not say so:\n%s", out)
	}
	// The built-ins and the flow tags are whisk's, so they are there regardless.
	for _, want := range []string{"required", "currency", "json             built in"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not carry %q:\n%s", want, out)
		}
	}
}

// A custom Unmarshal is reachable by -format, and one whose name a -format value
// cannot produce is not.
func TestTagReport_customFormat(t *testing.T) {
	root := t.TempDir()
	writeChecks(t, root, "pkg/scan", `package scan

func UnmarshalToml(data []byte, v any) error { return nil }
func UnmarshalJSON(data []byte, v any) error { return nil }
`)

	out := report(t, root, projectConfig{}, nil)
	if !strings.Contains(out, "toml             UnmarshalToml") {
		t.Errorf("the custom format is missing:\n%s", out)
	}
	if !strings.Contains(out, "UnmarshalJSON  — no -format value reaches this function") {
		t.Errorf("UnmarshalJSON is not flagged — json is answered by the preset:\n%s", out)
	}
}

func report(t *testing.T, root string, cfg projectConfig, set map[string]bool) string {
	t.Helper()
	return reportWith(t, root, cfg, set, scan.DefaultConfig(), validate.DefaultConfig())
}

func reportWith(t *testing.T, root string, cfg projectConfig, set map[string]bool,
	scanCfg scan.Config, validateCfg validate.Config) string {
	t.Helper()
	var b strings.Builder
	if err := writeTagReport(&b, root, cfg, set, scanCfg, validateCfg); err != nil {
		t.Fatalf("writeTagReport: %v", err)
	}
	return b.String()
}

func writeChecks(t *testing.T, root, dir, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(dir))
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "rules.go"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
