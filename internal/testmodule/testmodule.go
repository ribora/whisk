// Package testmodule writes a throwaway module for tests that need the loader to
// type-check real source.
package testmodule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The go.work points whisk at this checkout, so the loader needs neither the
// network nor a go.sum.
func New(t *testing.T, source string) string {
	t.Helper()

	dir := t.TempDir()
	root := checkoutRoot(t)
	// A workspace must be at least as new as every module in it, and a literal here
	// goes stale on the next bump.
	version := goDirective(t, root)

	files := map[string]string{
		"go.mod":    "module sample\n\ngo " + version + "\n\nrequire gitlab.com/gorib/whisk v0.0.0\n",
		"go.work":   "go " + version + "\n\nuse (\n\t.\n\t" + root + "\n)\n",
		"sample.go": source,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// Found by walking up, not by counting: the callers sit at different depths.
func checkoutRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if isWhisk(filepath.Join(dir, "go.mod")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no whisk go.mod above %s", dir)
		}
		dir = parent
	}
}

func isWhisk(path string) bool {
	body, err := os.ReadFile(path)
	return err == nil && strings.HasPrefix(string(body), "module gitlab.com/gorib/whisk\n")
}

func goDirective(t *testing.T, root string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read whisk go.mod: %v", err)
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		if version, ok := strings.CutPrefix(strings.TrimSpace(line), "go "); ok {
			return strings.TrimSpace(version)
		}
	}
	t.Fatalf("whisk go.mod has no go directive")
	return ""
}
