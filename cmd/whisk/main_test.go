package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// $GOFILE is what go:generate sets, so the no-argument path is the one every
// generated file actually takes.
func TestPickTarget(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		gofile  string
		want    string
		wantErr string
	}{
		{name: "explicit path", args: []string{"model.go"}, want: "model.go"},
		{name: "from go:generate", gofile: "model.go", want: "model.go"},
		{name: "argument beats the environment", args: []string{"other.go"}, gofile: "model.go", want: "other.go"},
		{name: "nothing to read", wantErr: "no input file"},
		{name: "two files", args: []string{"a.go", "b.go"}, wantErr: "at most one file"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("GOFILE", c.gofile)

			got, err := pickTarget(c.args)
			switch {
			case c.wantErr != "":
				if err == nil {
					t.Fatalf("got %q, want an error containing %q", got, c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("error %q does not contain %q", err, c.wantErr)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			case got != c.want:
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// The generator is run from the package directory but resolves conventions
// against the module root, so it has to find that root from wherever it starts.
func TestFindProjectRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/svc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(root, "domain", "user", "model")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := findProjectRoot(deep)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("got %q, want %q", got, root)
	}

	if _, err := findProjectRoot(t.TempDir()); err == nil {
		t.Error("a directory with no go.mod above it reported a root")
	}
}

func TestReadModuleName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(path, []byte("// a comment\n\nmodule example.com/svc\n\ngo 1.27.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readModuleName(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "example.com/svc" {
		t.Errorf("got %q, want example.com/svc", got)
	}

	empty := filepath.Join(dir, "empty.mod")
	if err := os.WriteFile(empty, []byte("go 1.27.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readModuleName(empty); err == nil {
		t.Error("a go.mod with no module directive was accepted")
	}
}

// The import path a plugin is handed has to match where the files are, or a
// generated file names a package that is not there.
func TestResolveDir(t *testing.T) {
	const module = "example.com/svc"

	abs, imp := resolveDir("/svc", module, "pkg/validate")
	if abs != filepath.Join("/svc", "pkg/validate") {
		t.Errorf("abs = %q", abs)
	}
	if imp != module+"/pkg/validate" {
		t.Errorf("import = %q, want %q", imp, module+"/pkg/validate")
	}

	// The root itself is the module, with nothing to append.
	if _, imp := resolveDir("/svc", module, "."); imp != module {
		t.Errorf("import = %q, want %q", imp, module)
	}
}
