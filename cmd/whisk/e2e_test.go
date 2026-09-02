package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The golden suite builds plugins directly, so nothing there covers main: flag
// parsing, whisk.yaml, and the resolveDir calls between them.

func whiskBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "whisk")
	build := exec.Command("go", "build", "-o", bin, "gitlab.com/gorib/whisk/cmd/whisk")
	build.Dir = caseDir(t, "")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building whisk: %v\n%s", err, out)
	}
	return bin
}

// The validator sits outside the default directory, so only configuration can
// connect it to the model.
func project(t *testing.T, storageDir string) string {
	t.Helper()
	root := t.TempDir()

	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("go.mod", "module example.com/svc\n\ngo 1.27.0\n")
	write("model/auth.go", "package model\n\ntype Auth struct {\n\tCode string `json:\"code\" validate:\"passcode\"`\n}\n")
	// Named for the package, not for the tag: the lookup is by func name.
	write(filepath.Join(storageDir, "rules.go"),
		"package checks\n\nfunc IsPasscode(v string, args ...string) bool { return len(v) == 6 }\n")

	return root
}

func runWhisk(t *testing.T, bin, root string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func generated(t *testing.T, root string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "model", "auth_whisk_gen.go"))
	if err != nil {
		t.Fatalf("no generated file: %v", err)
	}
	return string(body)
}

// The setting only exists if it survives the whole path into the emitted call.
func TestConfigFileReachesTheGenerator(t *testing.T) {
	bin := whiskBinary(t)
	root := project(t, "internal/checks")
	if err := os.WriteFile(filepath.Join(root, configFile),
		[]byte("validate:\n  storage: internal/checks\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := runWhisk(t, bin, root, "model/auth.go"); err != nil {
		t.Fatalf("whisk: %v\n%s", err, out)
	}
	body := generated(t, root)
	if !strings.Contains(body, "checks.IsPasscode") {
		t.Errorf("the configured validator was not called:\n%s", body)
	}
	if !strings.Contains(body, "example.com/svc/internal/checks") {
		t.Errorf("the configured dir did not become an import:\n%s", body)
	}
}

// The negative control: without the file the same project fails, so the test
// above is about the file and not about a default that happened to match.
func TestWithoutTheConfigFileTheTagIsUnresolved(t *testing.T) {
	bin := whiskBinary(t)
	root := project(t, "internal/checks")

	out, err := runWhisk(t, bin, root, "model/auth.go")
	if err == nil {
		t.Fatalf("expected a failure, got:\n%s", out)
	}
	for _, want := range []string{"Auth.Code", "passcode", "pkg/validate"} {
		if !strings.Contains(out, want) {
			t.Errorf("error does not mention %q:\n%s", want, out)
		}
	}
}

// Precedence, end to end: the file names one directory and the flag another.
func TestFlagBeatsTheConfigFile(t *testing.T) {
	bin := whiskBinary(t)
	root := project(t, "internal/checks")
	if err := os.WriteFile(filepath.Join(root, configFile),
		[]byte("validate:\n  storage: nowhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runWhisk(t, bin, root, "-validate.storage=internal/checks", "model/auth.go")
	if err != nil {
		t.Fatalf("whisk: %v\n%s", err, out)
	}
	if body := generated(t, root); !strings.Contains(body, "checks.IsPasscode") {
		t.Errorf("the flag lost to the file:\n%s", body)
	}
}

// init is a subcommand, not a target file, so it must not be parsed as one.
func TestInitSubcommand(t *testing.T) {
	bin := whiskBinary(t)
	root := project(t, "pkg/validate")

	if out, err := runWhisk(t, bin, root, "init"); err != nil {
		t.Fatalf("whisk init: %v\n%s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(root, configFile))
	if err != nil {
		t.Fatalf("init wrote nothing: %v", err)
	}
	if string(body) != configTemplate {
		t.Errorf("wrote:\n%s", body)
	}

	// The written file is the defaults, so generation has to work straight after.
	if out, err := runWhisk(t, bin, root, "model/auth.go"); err != nil {
		t.Fatalf("the file init wrote does not work: %v\n%s", err, out)
	}
	if body := generated(t, root); !strings.Contains(body, "checks.IsPasscode") {
		t.Errorf("the default storage dir was not used:\n%s", body)
	}
}
