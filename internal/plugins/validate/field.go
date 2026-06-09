package validate

import (
	"fmt"
	"go/types"
	"reflect"
	"strings"

	"gitlab.com/gorib/whisk/internal/shared"
)

type field struct {
	shared.FieldInfo

	// May carry a trailing synthetic ",dive" added by promote for struct-typed
	// fields, so nested validation runs without an explicit dive tag.
	ParsedTags []Tag
	// Set when that synthetic ",dive" was added; struct fields skip the
	// zero-value guard that wraps scalar checks.
	IsStruct bool
}

func promote(f shared.FieldInfo, pkgPath string) field {
	raw := f.Tag.Get("validate")
	hasDive := false
	for _, t := range strings.Split(raw, ",") {
		if strings.TrimSpace(t) == "dive" {
			hasDive = true
			break
		}
	}

	isStruct := false
	base := unwrapToBaseTypeText(f.TypeRef, pkgPath)
	if !hasDive && !isBasicType(base) && f.Type != "time.Time" && base != "" {
		if raw == "" {
			raw = "dive"
		} else {
			raw += ",dive"
		}
		isStruct = true
	}

	return field{
		FieldInfo:  f,
		ParsedTags: parseTags(raw),
		IsStruct:   isStruct,
	}
}

// effectiveName returns prefix+Name, prefixed with `*` when the field is
// a pointer (so callers can dereference into a value comparison).
func (f field) effectiveName(prefix string) string {
	name := prefix + f.Name
	if f.IsPointer {
		return "*" + name
	}
	return name
}

// zeroCondition returns the is-zero test for fieldName. ok=false means the
// type has no meaningful zero check; bool is a hard error (use equal, not
// required).
func (f field) zeroCondition(fieldName string) (string, bool, error) {
	if f.IsSlice || f.IsMap {
		return fmt.Sprintf("len(%s) == 0", fieldName), true, nil
	}
	if f.Type == "string" {
		return fmt.Sprintf("%s == \"\"", fieldName), true, nil
	}
	if f.IsPointer {
		return fmt.Sprintf("%s == nil", fieldName), true, nil
	}
	if f.Type == "time.Time" {
		return fmt.Sprintf("%s.IsZero()", fieldName), true, nil
	}
	if f.Type == "bool" {
		return "", false, fmt.Errorf("required tag doesn't make sense for bool field %q, use equal instead", f.Name)
	}
	if isBasicType(f.Type) {
		return fmt.Sprintf("%s == 0", fieldName), true, nil
	}
	return "", false, nil
}

func underlyingStructFields(st *types.Struct, pkgPath string) []field {
	out := make([]field, 0, st.NumFields())
	for i := 0; i < st.NumFields(); i++ {
		fv := st.Field(i)
		fi := shared.FieldInfo{
			Name:    fv.Name(),
			Tag:     reflect.StructTag(st.Tag(i)),
			Type:    typeText(fv.Type(), pkgPath),
			TypeRef: fv.Type(),
		}
		describeKind(&fi, fv.Type(), pkgPath)
		out = append(out, promote(fi, pkgPath))
	}
	return out
}

// Mirrors shared/parser.go's field-kind detection, kept private here.
func describeKind(fi *shared.FieldInfo, t types.Type, pkgPath string) {
	switch x := t.(type) {
	case *types.Pointer:
		fi.IsPointer = true
		if s, ok := x.Elem().(*types.Struct); ok {
			fi.AnonymousType = s
		}
	case *types.Struct:
		fi.AnonymousType = x
	case *types.Slice:
		fi.IsSlice = true
		fi.ElemType = typeText(x.Elem(), pkgPath)
		fi.AnonymousElem = anonStruct(x.Elem())
	case *types.Array:
		fi.IsSlice = true
		fi.ElemType = typeText(x.Elem(), pkgPath)
		fi.AnonymousElem = anonStruct(x.Elem())
	case *types.Map:
		fi.IsMap = true
		fi.KeyType = typeText(x.Key(), pkgPath)
		fi.ElemType = typeText(x.Elem(), pkgPath)
		fi.AnonymousElem = anonStruct(x.Elem())
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

func typeText(t types.Type, pkgPath string) string {
	switch x := t.(type) {
	case *types.Basic:
		return x.Name()
	case *types.Pointer:
		return "*" + typeText(x.Elem(), pkgPath)
	case *types.Slice:
		return "[]" + typeText(x.Elem(), pkgPath)
	case *types.Array:
		return fmt.Sprintf("[%d]%s", x.Len(), typeText(x.Elem(), pkgPath))
	case *types.Map:
		return "map[" + typeText(x.Key(), pkgPath) + "]" + typeText(x.Elem(), pkgPath)
	case *types.Named:
		obj := x.Obj()
		if obj.Pkg() == nil || obj.Pkg().Path() == pkgPath {
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

// isBasicType reports whether t names a predeclared type (leading "*" ignored).
func isBasicType(t string) bool {
	t = strings.TrimPrefix(t, "*")
	switch t {
	case "string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "bool", "byte", "rune":
		return true
	}
	return false
}

// unwrapToBaseTypeText peels pointer/slice/array/map down to the leaf type's text.
func unwrapToBaseTypeText(t types.Type, pkgPath string) string {
	for {
		switch x := t.(type) {
		case *types.Pointer:
			t = x.Elem()
		case *types.Slice:
			t = x.Elem()
		case *types.Array:
			t = x.Elem()
		case *types.Map:
			t = x.Elem()
		default:
			return typeText(t, pkgPath)
		}
	}
}
