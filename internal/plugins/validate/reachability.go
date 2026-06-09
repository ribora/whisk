package validate

import (
	"go/types"
	"reflect"
	"strings"

	"gitlab.com/gorib/whisk/internal/shared"
)

// willHaveValidate reports whether t will have a Validate() method at the
// final compile — i.e. it carries validate tags / a hook / a hand-written
// Validate, directly or through a nested field (embed, named struct, or
// slice/map/array/pointer element).
//
// It keys off struct tags and hand-written methods, both visible under
// -tags whisk, NOT off whisk-generated methods (which are excluded). Since a
// generated Validate exists only because of those same visible tags/hooks,
// the predicate has no false negatives, and it agrees across packages —
// every whisk run computes it identically. That makes it safe both as the
// "does this struct contribute?" test and as the "can I call v.Validate()
// directly?" test.
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

// isInterfaceType reports whether t's underlying type is an interface — those
// dive targets keep the runtime type-assertion, since their dynamic value may
// carry a Validate that the static type doesn't expose.
func isInterfaceType(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// diveElem returns the type a dive iterates over: the element of a
// slice/array/map, or the (pointer-unwrapped) struct type itself.
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
