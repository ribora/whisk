package shared

import (
	"cmp"
	"fmt"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"slices"

	"golang.org/x/tools/go/packages"
)

const BuildTag = "whisk"

// Source order is part of the contract: the output is compared against a golden.
func Load(pkgDir, targetFile string) (pkgName string, structs []StructInfo, err error) {
	pkg, err := loadPackage(pkgDir)
	if err != nil {
		return "", nil, err
	}
	structs, err = structsDeclaredIn(pkg, absPath(targetFile))
	if err != nil {
		return "", nil, err
	}
	return pkg.Name, structs, nil
}

// The build tag hides whisk's own previous output, so a stale generated file cannot
// block the run that would replace it.
//
// Only what is read: Name, PkgPath, Errors, Types, Fset. NeedTypesInfo alone cost
// 66MB of per-run memory for maps nothing here opens. NeedSyntax is not optional
// though — without it the loader answers from export data, and an unexported struct
// is not in export data.
func loadPackage(pkgDir string) (*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes |
			packages.NeedDeps | packages.NeedImports,
		Dir:        pkgDir,
		BuildFlags: []string{"-tags", BuildTag},
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package in %s", pkgDir)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("%s: %v", pkg.PkgPath, pkg.Errors[0])
	}
	if pkg.Types == nil {
		return nil, fmt.Errorf("no type info for package %s", pkg.PkgPath)
	}
	return pkg, nil
}

// The package scope is a map, so the result is ordered by declaration position:
// generated output is compared against a golden and cannot shuffle between runs.
func structsDeclaredIn(pkg *packages.Package, targetAbs string) ([]StructInfo, error) {
	type pending struct {
		info StructInfo
		pos  token.Pos
	}
	var collected []pending

	pkgPath := pkg.Types.Path()
	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		named, st, ok := namedStruct(scope.Lookup(name))
		if !ok {
			continue
		}
		declPos := named.Obj().Pos()
		if !samePath(pkg.Fset.Position(declPos).Filename, targetAbs) {
			continue
		}

		info, err := describe(name, pkgPath, named, st)
		if err != nil {
			return nil, err
		}
		collected = append(collected, pending{info: info, pos: declPos})
	}

	slices.SortFunc(collected, func(a, b pending) int { return cmp.Compare(a.pos, b.pos) })
	structs := make([]StructInfo, len(collected))
	for i, p := range collected {
		structs[i] = p.info
	}
	return structs, nil
}

func namedStruct(obj types.Object) (*types.Named, *types.Struct, bool) {
	tn, ok := obj.(*types.TypeName)
	if !ok {
		return nil, nil, false
	}
	named, ok := tn.Type().(*types.Named)
	if !ok {
		return nil, nil, false
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, nil, false
	}
	return named, st, true
}

func describe(name, pkgPath string, named *types.Named, st *types.Struct) (StructInfo, error) {
	hasValidate, err := ownHookSignature(named, "validate", "")
	if err != nil {
		return StructInfo{}, err
	}
	hasParse, err := ownHookSignature(named, "parse", "gitlab.com/gorib/whisk/scan.Source")
	if err != nil {
		return StructInfo{}, err
	}
	return StructInfo{
		Name:            name,
		PkgPath:         pkgPath,
		Fields:          Fields(st, pkgPath),
		HasValidateHook: hasValidate,
		HasParseHook:    hasParse,
		Named:           named,
	}, nil
}

// Fields describes the members of st. Both plugins need this, and the validate one
// used to carry its own copy of the loop.
func Fields(st *types.Struct, pkgPath string) []FieldInfo {
	out := make([]FieldInfo, 0, st.NumFields())
	for i := range st.NumFields() {
		f := st.Field(i)
		tag := reflect.StructTag(st.Tag(i))

		if f.Embedded() {
			_, isPtr := f.Type().(*types.Pointer)
			out = append(out, FieldInfo{
				Name:       f.Name(),
				Tag:        tag,
				Type:       TypeText(f.Type(), pkgPath),
				TypeRef:    f.Type(),
				IsEmbedded: true,
				IsPointer:  isPtr,
			})
			continue
		}

		fi := FieldInfo{
			Name:    f.Name(),
			Tag:     tag,
			Type:    TypeText(f.Type(), pkgPath),
			TypeRef: f.Type(),
		}

		DescribeKind(&fi, f.Type(), pkgPath)

		out = append(out, fi)
	}
	return out
}

func UnwrapNamed(t types.Type) (*types.Named, bool) {
	for {
		switch x := t.(type) {
		case *types.Named:
			return x, true
		case *types.Pointer:
			t = x.Elem()
		default:
			return nil, false
		}
	}
}

// absPath resolves p against the current working directory. targetFile is
// always relative to CWD (or already absolute) — it must NOT be re-joined
// with pkgDir, since pkgDir is derived from it (filepath.Dir) and doing so
// would double the directory component (e.g. "api/doc.go" → "api/api/doc.go").
func absPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	return abs
}

func samePath(a, b string) bool {
	return canonPath(a) == canonPath(b)
}

func canonPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}
