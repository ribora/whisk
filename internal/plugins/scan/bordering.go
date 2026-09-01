package scan

import (
	"fmt"
	"go/types"
	"reflect"
	"strings"

	"gitlab.com/gorib/whisk/internal/shared"
)

func checkBordering(s shared.StructInfo) error {
	for _, f := range s.Fields {
		raw := strings.TrimSpace(f.Tag.Get("scan"))
		if raw == "embed" || raw == "-" {
			continue
		}
		st, ok := underlyingStruct(f.TypeRef)
		if !ok {
			continue
		}
		if structHasBordering(st, map[*types.Struct]bool{}) {
			return fmt.Errorf("%s.%s: type %s has scan tags but is not marked scan:\"embed\" — its tags will not be populated", s.Name, f.Name, f.Type)
		}
	}
	return nil
}

func structHasBordering(st *types.Struct, seen map[*types.Struct]bool) bool {
	if seen[st] {
		return false
	}
	seen[st] = true
	for i := 0; i < st.NumFields(); i++ {
		raw := strings.TrimSpace(reflect.StructTag(st.Tag(i)).Get("scan"))
		switch raw {
		case "-", "":
			continue
		case "embed":
			inner, ok := underlyingStruct(st.Field(i).Type())
			if !ok {
				continue
			}
			if structHasBordering(inner, seen) {
				return true
			}
		default:
			return true
		}
	}
	return false
}

func underlyingStruct(t types.Type) (*types.Struct, bool) {
	if named, ok := shared.UnwrapNamed(t); ok {
		st, ok := named.Underlying().(*types.Struct)
		return st, ok
	}
	switch x := t.(type) {
	case *types.Struct:
		return x, true
	case *types.Pointer:
		return underlyingStruct(x.Elem())
	case *types.Slice:
		return underlyingStruct(x.Elem())
	case *types.Array:
		return underlyingStruct(x.Elem())
	}
	return nil, false
}
