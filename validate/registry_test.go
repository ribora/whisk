package validate_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ribora/whisk/internal/shared"
	"github.com/ribora/whisk/validate"
)

// Called by name from the emitter, not through BuiltIns.
var wiredDirectly = map[string]bool{
	"IsTimeMin": true,
	"IsTimeMax": true,
	"IsTimeGT":  true,
	"IsTimeLT":  true,
}

// results counts return values per Is* function in this package.
func results(t *testing.T) map[string]int {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	fset := token.NewFileSet()
	out := map[string]int{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "Is") {
				continue
			}
			n := 0
			if fn.Type.Results != nil {
				for _, f := range fn.Type.Results.List {
					n += max(len(f.Names), 1)
				}
			}
			out[fn.Name.Name] = n
		}
	}
	return out
}

// BuiltIns is a hand-kept map, and its value claims whether Is<Tag> hands back a
// reason. The emitter believes that claim: a wrong bool writes a call with the
// wrong number of results, and the defect only shows up in generated code. The
// two have to be checked against each other somewhere, and the compiler cannot.
func TestBuiltInsMatchTheFunctions(t *testing.T) {
	got := results(t)

	for tag, returnsError := range validate.BuiltIns {
		name := "Is" + shared.PascalCase(tag)
		n, ok := got[name]
		if !ok {
			t.Errorf("BuiltIns has %q, but %s is not declared in this package", tag, name)
			continue
		}
		want := 1
		if returnsError {
			want = 2
		}
		if n != want {
			t.Errorf("BuiltIns[%q] = %v, so %s should return %d value(s), it returns %d",
				tag, returnsError, name, want, n)
		}
	}

	for name := range got {
		if wiredDirectly[name] {
			continue
		}
		if _, ok := validate.BuiltIns[tagOf(name)]; !ok {
			t.Errorf("%s is exported but no tag reaches it — add it to BuiltIns or the generator will call it unsupported", name)
		}
	}
}

// The inverse of "Is" + PascalCase, which holds because every tag is one word.
func tagOf(fn string) string {
	return strings.ToLower(strings.TrimPrefix(fn, "Is"))
}

// The README used to carry a count of the built-ins, which was wrong by one and
// had nothing keeping it honest. `whisk tags` prints the real list, so the count
// is gone — but the README still names them for a reader who has no project to
// run it in, and that naming is what drifts when a built-in is added. This is
// the guard the number was pretending to be.
func TestReadmeNamesEveryBuiltIn(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "README.md"))
	if err != nil {
		t.Fatalf("read README: %v", err)
	}
	body := string(readme)

	for tag := range validate.BuiltIns {
		// Backticked, and either bare or carrying its argument syntax —
		// `datetime=<layout>` documents `datetime`.
		if strings.Contains(body, "`"+tag+"`") || strings.Contains(body, "`"+tag+"=") {
			continue
		}
		t.Errorf("BuiltIns has %q but the README never names it — add it to the tag list", tag)
	}
}
