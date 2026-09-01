package shared

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// The convention lookups — validate's Is<Tag>, scan's Unmarshal<Format> — emit a
// package-qualified call, so a method of the same name is not a match: accepting one
// would emit checks.IsPasscode(…) for something that is not there.
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
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name {
			return fn
		}
	}
	return nil
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
