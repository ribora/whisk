package shared

import (
	"fmt"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"sort"

	"golang.org/x/tools/go/packages"
)

const BuildTag = "whisk"

// Load returns every top-level struct declared in targetFile, in source order.
func Load(pkgDir, targetFile string) (pkgName string, structs []StructInfo, err error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedDeps | packages.NeedImports,
		Dir:        pkgDir,
		BuildFlags: []string{"-tags", BuildTag},
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return "", nil, err
	}
	if len(pkgs) == 0 {
		return "", nil, fmt.Errorf("no package in %s", pkgDir)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return "", nil, fmt.Errorf("%s: %v", pkg.PkgPath, pkg.Errors[0])
	}
	if pkg.Types == nil {
		return "", nil, fmt.Errorf("no type info for package %s", pkg.PkgPath)
	}

	targetAbs := absPath(targetFile)
	pkgPath := pkg.Types.Path()

	type pending struct {
		info StructInfo
		pos  token.Pos
	}
	var collected []pending

	scope := pkg.Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		tn, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		st, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		declPos := named.Obj().Pos()
		if !samePath(pkg.Fset.Position(declPos).Filename, targetAbs) {
			continue
		}

		hasValidate, err := ownHookSignature(named, "validate", "")
		if err != nil {
			return "", nil, err
		}
		hasParse, err := ownHookSignature(named, "parse", "gitlab.com/gorib/whisk/scan.Source")
		if err != nil {
			return "", nil, err
		}

		collected = append(collected, pending{
			info: StructInfo{
				Name:            name,
				PkgPath:         pkgPath,
				Fields:          extractFields(st, pkgPath),
				HasValidateHook: hasValidate,
				HasParseHook:    hasParse,
				Named:           named,
			},
			pos: declPos,
		})
	}

	sort.Slice(collected, func(i, j int) bool { return collected[i].pos < collected[j].pos })
	structs = make([]StructInfo, len(collected))
	for i, p := range collected {
		structs[i] = p.info
	}
	return pkg.Name, structs, nil
}

func extractFields(st *types.Struct, pkgPath string) []FieldInfo {
	out := make([]FieldInfo, 0, st.NumFields())
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		tag := reflect.StructTag(st.Tag(i))

		if f.Embedded() {
			_, isPtr := f.Type().(*types.Pointer)
			out = append(out, FieldInfo{
				Name:       f.Name(),
				Tag:        tag,
				Type:       typeText(f.Type(), pkgPath),
				TypeRef:    f.Type(),
				IsEmbedded: true,
				IsPointer:  isPtr,
			})
			continue
		}

		fi := FieldInfo{
			Name:    f.Name(),
			Tag:     tag,
			Type:    typeText(f.Type(), pkgPath),
			TypeRef: f.Type(),
		}

		switch t := f.Type().(type) {
		case *types.Pointer:
			fi.IsPointer = true
			if s, ok := t.Elem().(*types.Struct); ok {
				fi.AnonymousType = s
			}
		case *types.Struct:
			fi.AnonymousType = t
		case *types.Slice:
			fi.IsSlice = true
			fi.ElemType = typeText(t.Elem(), pkgPath)
			fi.AnonymousElem = anonStruct(t.Elem())
		case *types.Array:
			fi.IsSlice = true
			fi.ElemType = typeText(t.Elem(), pkgPath)
			fi.AnonymousElem = anonStruct(t.Elem())
		case *types.Map:
			fi.IsMap = true
			fi.KeyType = typeText(t.Key(), pkgPath)
			fi.ElemType = typeText(t.Elem(), pkgPath)
			fi.AnonymousElem = anonStruct(t.Elem())
		}

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

func anonStruct(t types.Type) *types.Struct {
	if ptr, ok := t.(*types.Pointer); ok {
		if s, ok := ptr.Elem().(*types.Struct); ok {
			return s
		}
		return nil
	}
	if s, ok := t.(*types.Struct); ok {
		return s
	}
	return nil
}

func typeText(t types.Type, currentPkgPath string) string {
	switch x := t.(type) {
	case *types.Basic:
		return x.Name()
	case *types.Pointer:
		return "*" + typeText(x.Elem(), currentPkgPath)
	case *types.Slice:
		return "[]" + typeText(x.Elem(), currentPkgPath)
	case *types.Array:
		return fmt.Sprintf("[%d]%s", x.Len(), typeText(x.Elem(), currentPkgPath))
	case *types.Map:
		return "map[" + typeText(x.Key(), currentPkgPath) + "]" + typeText(x.Elem(), currentPkgPath)
	case *types.Named:
		obj := x.Obj()
		if obj.Pkg() == nil || obj.Pkg().Path() == currentPkgPath {
			return obj.Name()
		}
		return obj.Pkg().Name() + "." + obj.Name()
	case *types.Struct:
		return "struct"
	case *types.Interface:
		return "interface"
	}
	return ""
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
