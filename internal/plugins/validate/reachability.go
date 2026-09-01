package validate

import (
	"go/types"
	"reflect"
	"strings"

	"gitlab.com/gorib/whisk/internal/shared"
)

// Whether t has a Validate at the final compile, decided from tags and
// hand-written methods rather than from generated ones — which are invisible
// under -tags whisk. A generated Validate exists only because of those same
// tags, so the answer has no false negatives and every run computes it alike.
func willHaveValidate(t types.Type) bool {
	return reachesValidation(t, map[*types.Named]bool{})
}

func reachesValidation(t types.Type, seen map[*types.Named]bool) bool {
	switch x := t.(type) {
	case *types.Pointer:
		return reachesValidation(x.Elem(), seen)
	case *types.Slice:
		return reachesValidation(x.Elem(), seen)
	case *types.Array:
		return reachesValidation(x.Elem(), seen)
	case *types.Map:
		return reachesValidation(x.Elem(), seen)
	case *types.Named:
		if seen[x] {
			return false
		}
		seen[x] = true
		if shared.HasOwnValidateMethod(x) || shared.HasOwnValidateHook(x) {
			return true
		}
		st, ok := x.Underlying().(*types.Struct)
		if !ok {
			return false
		}
		return structReaches(st, seen)
	case *types.Struct:
		return structReaches(x, seen)
	}
	return false
}

func structReaches(st *types.Struct, seen map[*types.Named]bool) bool {
	for i := 0; i < st.NumFields(); i++ {
		if strings.TrimSpace(reflect.StructTag(st.Tag(i)).Get("validate")) != "" {
			return true
		}
		if reachesValidation(st.Field(i).Type(), seen) {
			return true
		}
	}
	return false
}

// An interface keeps the run-time assertion: its dynamic value may carry a
// Validate the static type does not expose.
func isInterfaceType(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// What a dive iterates over: the element of a collection, or the value itself.
func diveElem(f field) types.Type {
	switch x := f.TypeRef.(type) {
	case *types.Slice:
		return x.Elem()
	case *types.Array:
		return x.Elem()
	case *types.Map:
		return x.Elem()
	case *types.Pointer:
		return x.Elem()
	}
	return f.TypeRef
}
