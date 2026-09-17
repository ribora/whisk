// The golden-file suite for whisk codegen. Each subdirectory under
// testdata/cases/ holds a self-contained case:
//
//	setup.go            — source struct definitions (no scan/validate
//	                      transformations applied yet)
//	golden_whisk_gen.go — expected rendered output (build-tagged
//	                      `!whisk` so it's excluded from the loader's
//	                      view at codegen time, matching production)
//
// `-update` regenerates the goldens after a deliberate change. This lives beside
// main.go rather than in testdata because the go tool skips testdata for wildcard
// patterns: from in there, `go test ./...` ran none of it.
package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/gorib/whisk/internal/generator"
	"gitlab.com/gorib/whisk/internal/plugins/scan"
	"gitlab.com/gorib/whisk/internal/plugins/validate"
	"gitlab.com/gorib/whisk/internal/shared"
)

var update = flag.Bool("update", false, "rewrite golden_whisk_gen.go files instead of comparing")

// caseSpec describes one golden test case.
type caseSpec struct {
	dir     string
	format  scan.Format
	storage string // sub-dir under the case dir; "" means no custom storage
	checks  string // import path for user-defined checks (empty if storage empty)
}

var successCases = []caseSpec{
	{dir: "basic", format: scan.FormatJSON},
	{dir: "named_scalars", format: scan.FormatJSON},
	{dir: "narrow_scalars", format: scan.FormatJSON},
	{dir: "zero_guards", format: scan.FormatJSON},
	{dir: "text_unmarshal", format: scan.FormatJSON},
	{dir: "embed", format: scan.FormatJSON},
	{dir: "dive_slice", format: scan.FormatJSON},
	{dir: "dive_map", format: scan.FormatJSON},
	{dir: "hook_only", format: scan.FormatJSON},
	{dir: "tags_only", format: scan.FormatJSON},
	{dir: "required_struct_value", format: scan.FormatJSON},
	{dir: "xml_body", format: scan.FormatXML},
	{
		dir:     "custom_validator",
		format:  scan.FormatJSON,
		storage: "checks",
		checks:  "gitlab.com/gorib/whisk/testdata/cases/custom_validator/checks",
	},
	{
		dir: "custom_format",
		format: scan.Format{
			Tag: "toml",
			Import: generator.Import{
				Alias: "bodyfmt",
				Path:  "gitlab.com/gorib/whisk/testdata/cases/custom_format/extractor",
			},
			Unmarshal: "bodyfmt.UnmarshalToml",
		},
	},
}

type errorCase struct {
	dir         string
	errContains string
}

var errorCases = []errorCase{
	{dir: "err_bordering", errContains: `not marked scan:"embed"`},
	{dir: "err_bad_tag", errContains: `unknown validator tag "bogusvalidator"`},
	{dir: "err_pointer_embed", errContains: "on a pointer field"},
	{dir: "err_time_eq", errContains: "doesn't make sense for time field"},
}

func TestSuccess(t *testing.T) {
	for _, c := range successCases {
		t.Run(c.dir, func(t *testing.T) {
			pkgDir := caseDir(t, c.dir)
			input := filepath.Join(pkgDir, "setup.go")

			pkgName, structs, err := shared.Load(pkgDir, input)
			if err != nil {
				t.Fatalf("shared.Load: %v", err)
			}

			plugins := []generator.Plugin{
				scan.New(c.format),
				validate.New(validate.Options{
					StoragePath:      storagePath(pkgDir, c.storage),
					UserChecksImport: c.checks,
					ExtractTags:      []string{c.format.Tag},
				}),
			}

			got, _, err := generator.Render(pkgName, structs, plugins)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			goldenPath := filepath.Join(pkgDir, "golden_whisk_gen.go")
			if *update {
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden (run with -update to create): %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s",
					c.dir, got, want)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	for _, c := range errorCases {
		t.Run(c.dir, func(t *testing.T) {
			pkgDir := caseDir(t, c.dir)
			input := filepath.Join(pkgDir, "setup.go")

			pkgName, structs, err := shared.Load(pkgDir, input)
			if err != nil {
				// Some error cases (e.g. compile errors in setup.go) bail
				// out at the load stage rather than the plugin stage. That
				// counts as a passing test as long as the message matches.
				if strings.Contains(err.Error(), c.errContains) {
					return
				}
				t.Fatalf("Load: %v (want error containing %q)", err, c.errContains)
			}
			plugins := []generator.Plugin{
				scan.New(scan.FormatJSON),
				validate.New(validate.Options{ExtractTags: []string{"json"}}),
			}
			_, _, err = generator.Render(pkgName, structs, plugins)
			if err == nil {
				t.Fatalf("expected error containing %q, got none", c.errContains)
			}
			if !strings.Contains(err.Error(), c.errContains) {
				t.Errorf("error mismatch: got %v, want substring %q", err, c.errContains)
			}
		})
	}
}

// Cases carrying their own _test.go: compiling proves the imports, running
// proves the code. A nil receiver in a generated loop compiles fine and only
// fails when something calls it.
var runnableCases = []string{
	"named_scalars",
	"narrow_scalars",
	"text_unmarshal",
	"zero_guards",
}

// TestGoldensRun executes those cases. The go tool skips testdata for wildcard
// patterns, so the import paths are named explicitly.
func TestGoldensRun(t *testing.T) {
	args := []string{"test", "-count=1"}
	for _, dir := range runnableCases {
		args = append(args, "gitlab.com/gorib/whisk/testdata/cases/"+dir)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = caseDir(t, "")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running generated cases: %v\n%s", err, out)
	}
}

// TestSuccess compares bytes and never builds what it generated, so a wrong
// import list passes it and still emits a file that does not compile — which is
// how named-type conversions once reached a golden without "strconv". The go
// tool skips testdata for wildcard patterns, hence the explicit import paths.
func TestGoldensCompile(t *testing.T) {
	args := []string{"build"}
	for _, c := range successCases {
		args = append(args, "gitlab.com/gorib/whisk/testdata/cases/"+c.dir)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = caseDir(t, "")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compiling goldens: %v\n%s", err, out)
	}
}

// caseDir returns the absolute path of a case directory under testdata/cases,
// found from the module root so the test works regardless of cwd.
func caseDir(t *testing.T, dir string) string {
	t.Helper()
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "testdata", "cases", dir)
}

func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.mod not found")
		}
		dir = parent
	}
}

func storagePath(pkgDir, sub string) string {
	if sub == "" {
		return ""
	}
	return filepath.Join(pkgDir, sub)
}
