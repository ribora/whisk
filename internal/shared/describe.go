package shared

import (
	"fmt"
	"go/types"
)

// How a Go type is described to the plugins. This lived here and, in a copy, in the
// validate plugin — until the copies disagreed about arrays.
func DescribeKind(fi *FieldInfo, t types.Type, pkgPath string) {
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
		fi.ElemType = TypeText(x.Elem(), pkgPath)
		fi.AnonymousElem = AnonStruct(x.Elem())
	case *types.Array:
		fi.IsSlice = true
		fi.IsArray = true
		fi.ElemType = TypeText(x.Elem(), pkgPath)
		fi.AnonymousElem = AnonStruct(x.Elem())
	case *types.Map:
		fi.IsMap = true
		fi.KeyType = TypeText(x.Key(), pkgPath)
		fi.ElemType = TypeText(x.Elem(), pkgPath)
		fi.AnonymousElem = AnonStruct(x.Elem())
	}
}

// AnonStruct is the inline struct behind t, through one pointer, or nil.
func AnonStruct(t types.Type) *types.Struct {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	s, _ := t.(*types.Struct)
	return s
}

// TypeText spells t as the generated file will: qualified against pkgPath, so a
// type from the same package keeps its bare name.
func TypeText(t types.Type, pkgPath string) string {
	switch x := t.(type) {
	case *types.Basic:
		return x.Name()
	case *types.Pointer:
		return "*" + TypeText(x.Elem(), pkgPath)
	case *types.Slice:
		return "[]" + TypeText(x.Elem(), pkgPath)
	case *types.Array:
		return fmt.Sprintf("[%d]%s", x.Len(), TypeText(x.Elem(), pkgPath))
	case *types.Map:
		return "map[" + TypeText(x.Key(), pkgPath) + "]" + TypeText(x.Elem(), pkgPath)
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
