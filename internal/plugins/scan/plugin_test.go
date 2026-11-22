package scan_test

import (
	"go/format"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ribora/whisk/internal/generator"
	"github.com/ribora/whisk/internal/plugins/scan"
	"github.com/ribora/whisk/internal/shared"
	"github.com/ribora/whisk/internal/testmodule"
)

// runScan loads the synthetic module, runs the scan plugin in isolation,
// and returns its emitted code (un-rendered — no package header / imports).
func runScan(t *testing.T, source string, format scan.Format) generator.Section {
	t.Helper()
	dir := testmodule.New(t, source)
	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatalf("shared.Load: %v", err)
	}
	sec, err := scan.New(format).Generate(structs)
	if err != nil {
		t.Fatalf("scan.Generate: %v", err)
	}
	return sec
}

// runScanExpectError loads + runs and expects a non-nil error containing want.
func runScanExpectError(t *testing.T, source, want string) {
	t.Helper()
	dir := testmodule.New(t, source)
	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatalf("shared.Load: %v", err)
	}
	_, err = scan.New(scan.FormatJSON).Generate(structs)
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got: %v", want, err)
	}
}

func TestGenerate_skipsBodyUnmarshalWhenNoBucket1Field(t *testing.T) {
	// A struct with no untagged fields (bucket-1 empty) has nothing for
	// json.Unmarshal to populate — the body block must be elided, and the
	// encoding/json import must not be pulled in.
	sec := runScan(t, `package sample
type Empty struct{}
`, scan.FormatJSON)

	must(t, string(sec.Code), "func (s *Empty) Parse(src scan.Source) error")
	mustNot(t, string(sec.Code), "src.Body()", "json.Unmarshal(body, s)")
	wantImports(t, sec.Imports, "github.com/ribora/whisk/scan")
}

func TestGenerate_scalarLookups(t *testing.T) {
	sec := runScan(t, `package sample
type Req struct {
	UserID  string `+"`scan:\"path=userId\"`"+`
	Page    int    `+"`scan:\"query=page\"`"+`
	Token   string `+"`scan:\"header=Authorization\"`"+`
	Active  bool   `+"`scan:\"query=active\"`"+`
}
`, scan.FormatJSON)

	code := string(sec.Code)
	must(t, code,
		`src.Lookup("path", "userId")`,
		`src.Lookup("query", "page")`,
		`src.Lookup("header", "Authorization")`,
		`src.Lookup("query", "active")`,
		`strconv.ParseInt(v, 10, 0)`, // 0 is strconv's platform width
		`strconv.ParseBool(v)`,
		`s.UserID = v`,
	)
	wantImports(t, sec.Imports, "github.com/ribora/failure", "github.com/ribora/whisk/scan", "strconv")
}

func TestGenerate_sliceRepeated(t *testing.T) {
	sec := runScan(t, `package sample
type Req struct {
	Tags []string `+"`scan:\"query=tag,repeated\"`"+`
}
`, scan.FormatJSON)
	code := string(sec.Code)
	must(t, code,
		`src.LookupAll("query", "tag")`,
		"strings.TrimSpace",
		"make([]string, 0, len(vs))",
	)
	wantImports(t, sec.Imports, "github.com/ribora/whisk/scan", "strings")
}

func TestGenerate_sliceCommaSplit(t *testing.T) {
	sec := runScan(t, `package sample
type Req struct {
	IDs []int `+"`scan:\"query=ids\"`"+`
}
`, scan.FormatJSON)
	code := string(sec.Code)
	must(t, code,
		`src.Lookup("query", "ids")`,
		`strings.Split(v, ",")`,
		"strconv.ParseInt(p, 10, 0)",
	)
	wantImports(t, sec.Imports, "github.com/ribora/failure", "github.com/ribora/whisk/scan", "strconv", "strings")
}

func TestGenerate_embedDelegate(t *testing.T) {
	sec := runScan(t, `package sample
type Auth struct {
	Token string `+"`scan:\"header=Authorization\"`"+`
}
type Req struct {
	A      Auth  `+"`scan:\"embed\"`"+`
	UserID string `+"`scan:\"path=userId\"`"+`
}
`, scan.FormatJSON)
	code := string(sec.Code)
	must(t, code, "s.A.Parse(src)")
	if len(sec.Warnings) != 0 {
		t.Errorf("named-field embeds should not warn, got: %v", sec.Warnings)
	}
}

// The parent's decode skips an embed, so the name would be silently unread.
func TestGenerate_embedWithFormatNameErrors(t *testing.T) {
	runScanExpectError(t, `package sample
type Page struct {
	Cursor string `+"`json:\"cursor\"`"+`
}
type Req struct {
	Page Page   `+"`scan:\"embed\" json:\"page\"`"+`
	Name string `+"`json:\"name\"`"+`
}
`, `scan:"embed" and json:"page" contradict each other`)
}

// The name is what conflicts. `-` says "not in the body", which is what a flat
// mixin means, and an option without a name renames nothing.
func TestGenerate_embedWithFormatDashIsFine(t *testing.T) {
	for _, tag := range []string{`scan:"embed" json:"-"`, `scan:"embed" json:",omitempty"`} {
		sec := runScan(t, `package sample
type Page struct {
	Cursor string `+"`json:\"cursor\"`"+`
}
type Req struct {
	Page Page   `+"`"+tag+"`"+`
	Name string `+"`json:\"name\"`"+`
}
`, scan.FormatJSON)
		must(t, string(sec.Code), "s.Page.Parse(src)")
	}
}

// The tag whisk reads follows -format, so xml is checked against xml.
func TestGenerate_embedWithFormatNameFollowsTheFormat(t *testing.T) {
	// json is not the body format here, so its name says nothing about the body.
	sec := runScan(t, `package sample
type Page struct {
	Cursor string `+"`xml:\"cursor\"`"+`
}
type Req struct {
	Page Page   `+"`scan:\"embed\" json:\"page\"`"+`
	Name string `+"`xml:\"name\"`"+`
}
`, scan.FormatXML)
	must(t, string(sec.Code), "s.Page.Parse(src)")
}

func TestGenerate_selfDecodingParentKeepsWholeBody(t *testing.T) {
	sec := runScan(t, `package sample
type Auth struct {
	Token string `+"`scan:\"header=Authorization\"`"+`
}
type Req struct {
	A    Auth   `+"`scan:\"embed\"`"+`
	Name string `+"`json:\"name\"`"+`
}
func (r *Req) UnmarshalJSON([]byte) error { return nil }
`, scan.FormatJSON)
	code := string(sec.Code)
	must(t, code, "json.Unmarshal(body, s)")
	mustNot(t, code, "payload")
}

func TestGenerate_embedDelegateAfterBody(t *testing.T) {
	// Security regression (IDOR): when body unmarshal IS emitted (the
	// struct has a bucket-1 field), it must run BEFORE the embed delegate.
	// Otherwise a request body could override a field an embed populated
	// from a non-body scope (e.g. a principal sourced from the auth
	// context). See renderOne in render.go.
	sec := runScan(t, `package sample
type Auth struct {
	Token string `+"`scan:\"header=Authorization\"`"+`
}
type Req struct {
	Note   string `+"``"+`
	A      Auth   `+"`scan:\"embed\"`"+`
	UserID string `+"`scan:\"path=userId\"`"+`
}
`, scan.FormatJSON)

	// Scope the order check to Req.Parse — Auth.Parse (emitted first, since
	// Auth is declared first) has its own no-body path that would otherwise
	// confuse the index comparison.
	code := string(sec.Code)
	start := strings.Index(code, "func (s *Req) Parse")
	if start < 0 {
		t.Fatalf("missing Req.Parse in:\n%s", code)
	}
	req := code[start:]

	body := strings.Index(req, "json.Unmarshal(body, ")
	valEmbed := strings.Index(req, "s.A.Parse(src)")
	if body < 0 || valEmbed < 0 {
		t.Fatalf("expected body unmarshal and embed delegate in Req.Parse:\n%s", req)
	}
	if body > valEmbed {
		t.Fatalf("body unmarshal must precede embed delegate, got body@%d > embed@%d:\n%s",
			body, valEmbed, req)
	}
}

func TestGenerate_embedOnPointerErrors(t *testing.T) {
	// Pointer-embed used to rely on the parent's body unmarshal to allocate
	// the indirection — fragile, and broken once body becomes conditional.
	// Walk now rejects it outright; users wanting optional embeds must own
	// the allocation in a parse() hook.
	runScanExpectError(t, `package sample
type Auth struct {
	Token string `+"`scan:\"header=Authorization\"`"+`
}
type Req struct {
	A *Auth `+"`scan:\"embed\"`"+`
}
`, "scan:\"embed\" on a pointer field")
}

func TestGenerate_anonymousEmbedWarnsButDelegates(t *testing.T) {
	// Anonymous embed with scan:"embed" still works (delegates the same
	// way), but the embedded type's Parse promotes into the parent — so it
	// warns rather than errors.
	sec := runScan(t, `package sample
type Auth struct {
	Token string `+"`scan:\"header=Authorization\"`"+`
}
type Req struct {
	Auth `+"`scan:\"embed\"`"+`
	UserID string `+"`scan:\"path=userId\"`"+`
}
`, scan.FormatJSON)

	must(t, string(sec.Code), "s.Auth.Parse(src)")
	if len(sec.Warnings) == 0 {
		t.Fatal("expected a warning for anonymous embed, got none")
	}
	if !strings.Contains(sec.Warnings[0], "embedded field Auth") {
		t.Errorf("warning should name the embedded field, got: %q", sec.Warnings[0])
	}
}

func TestGenerate_embedOnInlineAnonStructErrors(t *testing.T) {
	// An inline anonymous struct can't carry a Parse method, so embed has
	// nothing to delegate to — this is a genuine misuse, not a style nit.
	runScanExpectError(t, `package sample
type Req struct {
	Inner struct{ ID string `+"`scan:\"path=id\"`"+` } `+"`scan:\"embed\"`"+`
}
`, "requires a named struct type")
}

func TestGenerate_parseHook(t *testing.T) {
	sec := runScan(t, `package sample

import "github.com/ribora/whisk/scan"

type Hooked struct {
	X string `+"`scan:\"query=x\"`"+`
}

func (s *Hooked) parse(_ scan.Source) error { return nil }
`, scan.FormatJSON)
	code := string(sec.Code)
	must(t, code, "if err := s.parse(src);")
}

func TestGenerate_skipTag(t *testing.T) {
	sec := runScan(t, `package sample
type Req struct {
	Ignored string `+"`scan:\"-\"`"+`
}
`, scan.FormatJSON)
	code := string(sec.Code)
	if strings.Contains(code, "Ignored") {
		t.Fatalf("scan:\"-\" field should not appear in output:\n%s", code)
	}
}

func TestGenerate_xmlFormat(t *testing.T) {
	sec := runScan(t, `package sample
type Req struct {
	Name string `+"`xml:\"name\"`"+`
}
`, scan.FormatXML)
	code := string(sec.Code)
	must(t, code, "xml.Unmarshal(body, s)")
	wantImports(t, sec.Imports, "encoding/xml", "github.com/ribora/failure", "github.com/ribora/whisk/scan")
}

func TestGenerate_formattedOutputCompiles(t *testing.T) {
	// Sanity check: the orchestrator runs go/format on the merged file. The
	// scan plugin's section must at least parse as Go after we wrap it in a
	// package + imports stub.
	sec := runScan(t, `package sample
type Req struct {
	UserID string  `+"`scan:\"path=userId\"`"+`
	Tags   []int   `+"`scan:\"query=tags,repeated\"`"+`
	Active *bool   `+"`scan:\"query=active\"`"+`
}
`, scan.FormatJSON)

	header := "package sample\n\nimport (\n"
	for _, imp := range importPaths(sec.Imports) {
		header += "\t\"" + imp + "\"\n"
	}
	header += ")\n\n"
	src := header + string(sec.Code)
	if _, err := format.Source([]byte(src)); err != nil {
		t.Fatalf("emitted code doesn't parse:\n%s\nerr: %v", src, err)
	}
}

func TestGenerate_borderingCheck(t *testing.T) {
	src := `package sample
type Inner struct {
	ID string ` + "`scan:\"path=id\"`" + `
}
type Outer struct {
	Inner Inner
}
`
	runScanExpectError(t, src, "not marked scan:\"embed\"")
}

func TestGenerate_borderingCheckSkippedForEmbed(t *testing.T) {
	src := `package sample
type Inner struct {
	ID string ` + "`scan:\"path=id\"`" + `
}
type Outer struct {
	Inner Inner ` + "`scan:\"embed\"`" + `
}
`
	sec := runScan(t, src, scan.FormatJSON)
	code := string(sec.Code)
	must(t, code, "s.Inner.Parse(src)")
}

func TestGenerate_borderingCheckSkippedForDash(t *testing.T) {
	src := `package sample
type Inner struct {
	ID string ` + "`scan:\"path=id\"`" + `
}
type Outer struct {
	Inner Inner ` + "`scan:\"-\"`" + `
}
`
	sec := runScan(t, src, scan.FormatJSON)
	code := string(sec.Code)
	if strings.Contains(code, "s.Inner.Parse(src)") {
		t.Fatalf("scan:\"-\" should not delegate:\n%s", code)
	}
}

func TestGenerate_brokenEmbedChainErrors(t *testing.T) {
	// embed → json → embed → path: Root delegates to Wrap, but Wrap reaches
	// Mid (which has scan tags via its own embed→path) through a json field,
	// not an embed. Mid.Deep's path tag would never populate ⇒ error.
	src := `package sample
type Deep struct {
	ID string ` + "`scan:\"path=id\"`" + `
}
type Mid struct {
	Deep Deep ` + "`scan:\"embed\"`" + `
}
type Wrap struct {
	Mid Mid ` + "`json:\"mid\"`" + `
}
type Root struct {
	Wrap Wrap ` + "`scan:\"embed\"`" + `
}
`
	runScanExpectError(t, src, "not marked scan:\"embed\"")
}

func TestResolveFormat_builtins(t *testing.T) {
	for _, name := range []string{"", "json", "xml"} {
		f, err := scan.ResolveFormat(name, scan.Extractor{})
		if err != nil {
			t.Fatalf("ResolveFormat(%q): %v", name, err)
		}
		if f.Unmarshal == "" || strings.Contains(f.Unmarshal, "bodyfmt") {
			t.Errorf("ResolveFormat(%q) should be a builtin, got %+v", name, f)
		}
	}
}

// The extractor is found by func name; the file name carries nothing.
func TestResolveFormat_customExtractor(t *testing.T) {
	for _, file := range []string{"toml.go", "formats.go"} {
		t.Run(file, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, file),
				[]byte("package x\nfunc UnmarshalToml(b []byte, v any) error { return nil }\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			f, err := scan.ResolveFormat("toml", scan.Extractor{Dir: dir, Label: "pkg/scan", Import: "mod/pkg/scan"})
			if err != nil {
				t.Fatalf("ResolveFormat: %v", err)
			}
			want := generator.Import{Alias: "bodyfmt", Path: "mod/pkg/scan"}
			if f.Tag != "toml" || f.Import != want || f.Unmarshal != "bodyfmt.UnmarshalToml" {
				t.Errorf("unexpected Format: %+v", f)
			}
		})
	}
}

// Named as configured ("pkg/scan"), not as the abs path nobody typed.
func TestResolveFormat_unknownFormatErrors(t *testing.T) {
	_, err := scan.ResolveFormat("toml", scan.Extractor{Dir: t.TempDir(), Label: "pkg/scan", Import: "mod/pkg/scan"})
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"UnmarshalToml", "pkg/scan", "no Unmarshal* funcs there"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestResolveFormat_missingExtractorDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	_, err := scan.ResolveFormat("toml", scan.Extractor{Dir: dir, Label: "pkg/scan", Import: "mod/pkg/scan"})
	if err == nil || !strings.Contains(err.Error(), "directory does not exist") {
		t.Fatalf("expected the error to report the missing directory, got %v", err)
	}
}

// A broken file used to read as "format not found" — the wrong bug to chase.
func TestResolveFormat_unparseableExtractor(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "toml.go"), []byte("package x\nfunc UnmarshalToml("), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := scan.ResolveFormat("toml", scan.Extractor{Dir: dir, Label: "pkg/scan", Import: "mod/pkg/scan"})
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("expected a parse error, got %v", err)
	}
}

func TestGenerate_invalidTagFormat(t *testing.T) {
	src := `package sample
type Req struct {
	X string ` + "`scan:\"justname\"`" + `
}
`
	runScanExpectError(t, src, "must be \"-\"")
}

func TestGenerate_unknownOption(t *testing.T) {
	src := `package sample
type Req struct {
	X []string ` + "`scan:\"query=x,foo\"`" + `
}
`
	runScanExpectError(t, src, "unknown scan option")
}

func TestGenerate_repeatedOnNonSlice(t *testing.T) {
	src := `package sample
type Req struct {
	X string ` + "`scan:\"query=x,repeated\"`" + `
}
`
	runScanExpectError(t, src, "requires a slice field")
}

func TestGenerate_embedOnNonStruct(t *testing.T) {
	src := `package sample
type Req struct {
	X string ` + "`scan:\"embed\"`" + `
}
`
	runScanExpectError(t, src, "requires a named struct type")
}

// must reports a failure if any needle is missing from haystack.
func must(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(haystack, n) {
			t.Errorf("missing %q in:\n%s", n, haystack)
		}
	}
}

func mustNot(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			t.Errorf("esists but should be missed %q in:\n%s", n, haystack)
		}
	}
}

func wantImports(t *testing.T, got []generator.Import, want ...string) {
	t.Helper()
	paths := importPaths(got)
	have := map[string]bool{}
	for _, p := range paths {
		have[p] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("missing import %q (got %v)", w, paths)
		}
	}
	for _, p := range paths {
		if !slices.Contains(want, p) {
			t.Errorf("unexpected extra import %q (want %v)", p, want)
		}
	}
}

// The assertions are about which packages the section pulls in, not about how each
// one is spelled.
func importPaths(imports []generator.Import) []string {
	out := make([]string, len(imports))
	for i, imp := range imports {
		out[i] = imp.Path
	}
	return out
}
