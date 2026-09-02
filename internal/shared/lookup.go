package shared

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Methods are left out: the lookups emit a package-qualified call, so accepting
// one would name something that is not there.
type FuncIndex map[string]*ast.FuncDecl

// A missing directory is an empty index — only the caller knows whether anything
// was looking. A file that does not parse is an error: it is ordinary project
// code, and reporting it as "tag not found" sends the author after the wrong bug.
func IndexFuncs(dir string) (FuncIndex, error) {
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		if st, statErr := os.Stat(dir); statErr == nil && !st.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", dir)
		}
		return nil, err
	}

	idx := FuncIndex{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		node, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, decl := range node.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				idx[fn.Name.Name] = fn
			}
		}
	}
	return idx, nil
}

func (i FuncIndex) withPrefix(prefix string) []string {
	var out []string
	for name := range i {
		if strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// The directory is a convention the author never spelled out, so a failed lookup
// that omits what is actually there reads as magic.
func SearchNote(dir, prefix string, idx FuncIndex) string {
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return "(directory does not exist)"
	}
	if names := idx.withPrefix(prefix); len(names) > 0 {
		return fmt.Sprintf("(found there: %s)", strings.Join(names, ", "))
	}
	return fmt.Sprintf("(no %s* funcs there)", prefix)
}

// company_id -> CompanyId
func PascalCase(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
