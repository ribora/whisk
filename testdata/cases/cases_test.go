// Package cases_test drives the golden-file suite for whisk codegen.
// Each subdirectory under testdata/cases/ holds a self-contained case:
//
//	setup.go            — source struct definitions (no scan/validate
//	                      transformations applied yet)
//	golden_whisk_gen.go — expected rendered output (build-tagged
//	                      `!whisk` so it's excluded from the loader's
//	                      view at codegen time, matching production)
//
// Run `go test ./whisk/testdata/cases/...` to verify, or with `-update`
// to regenerate the goldens after a deliberate change.
package cases_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"runtime"
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
	{dir: "embed", format: scan.FormatJSON},
	{dir: "dive_slice", format: scan.FormatJSON},
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
			Tag:       "toml",
			Import:    "bodyfmt gitlab.com/gorib/whisk/testdata/cases/custom_format/extractor",
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
	{dir: "err_bad_tag", errContains: "unsupported validator tag"},
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

// caseDir returns the absolute path of a case directory under testdata/.
// It uses the caller's source path so the test works regardless of cwd.
func caseDir(t *testing.T, dir string) string {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(here), dir)
}

func storagePath(pkgDir, sub string) string {
	if sub == "" {
		return ""
	}
	return filepath.Join(pkgDir, sub)
}
