package shared

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// FindFunc returns the top-level func named name declared in dir/file, or nil
// if the directory, file, or function is absent. Callers inspect the returned
// declaration for signature details. Used by the convention-directory lookups
// (validate's Is<Tag>, scan's Unmarshal<Format>).
func FindFunc(dir, file, name string) *ast.FuncDecl {
	if dir == "" {
		return nil
	}
	target := filepath.Join(dir, file)
	if _, err := os.Stat(target); err != nil {
		return nil
	}
	node, err := parser.ParseFile(token.NewFileSet(), target, nil, 0)
	if err != nil {
		return nil
	}
	for _, decl := range node.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// PascalCase converts snake_case → PascalCase ("company_id" → "CompanyId").
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
