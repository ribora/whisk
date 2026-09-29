package scan

import (
	"go/types"
	"slices"
	"strings"

	"gitlab.com/gorib/whisk/internal/shared"
)

// The strconv call is chosen by this. Empty for anything not basic, and the
// field then falls through to the marker.
func basicKind(t types.Type) string {
	if t == nil {
		return ""
	}
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return ""
	}
	return basic.Name()
}

// Only types the emitter writes out by name. A value decoded through
// UnmarshalText names none: the call goes through the field.
func spelledOutTypes(sf sourceField, f shared.FieldInfo, pkgPath string) []string {
	var refs []string
	add := func(t types.Type) {
		named, ok := shared.UnwrapNamed(t)
		if !ok {
			return
		}
		pkg := named.Obj().Pkg()
		if pkg == nil || pkg.Path() == pkgPath {
			return
		}
		refs = append(refs, pkg.Path())
	}

	switch {
	case sf.ElemType != "":
		add(sliceElem(f.TypeRef))
	case sf.Pointee != "":
		add(f.TypeRef)
	case !sf.Text && sf.Type != sf.Kind:
		add(f.TypeRef)
	}
	return refs
}

// The method is nearly always on the pointer receiver, so a value field is asked
// about *T and a pointer field about itself.
func decodesText(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, isPtr := t.(*types.Pointer); !isPtr {
		t = types.NewPointer(t)
	}
	ms := types.NewMethodSet(t)
	for i := range ms.Len() {
		fn, ok := ms.At(i).Obj().(*types.Func)
		if !ok || fn.Name() != "UnmarshalText" {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok || sig.Params().Len() != 1 || sig.Results().Len() != 1 {
			continue
		}
		slice, ok := sig.Params().At(0).Type().(*types.Slice)
		if !ok {
			continue
		}
		basic, ok := slice.Elem().(*types.Basic)
		if !ok || basic.Kind() != types.Uint8 {
			continue
		}
		return sig.Results().At(0).Type().String() == "error"
	}
	return false
}

// The method convention, UnmarshalJSON or UnmarshalYAML — not an extractor's UnmarshalToml.
func decodesItself(named *types.Named, formatTag string) bool {
	if named == nil {
		return false
	}
	ms := types.NewMethodSet(types.NewPointer(named))
	return ms.Lookup(nil, "Unmarshal"+strings.ToUpper(formatTag)) != nil
}

func bodyShape(fields []shared.FieldInfo, pkgPath string) ([]shapeField, []string) {
	var refs []string
	qualify := func(p *types.Package) string {
		if p.Path() == pkgPath {
			return ""
		}
		if !slices.Contains(refs, p.Path()) {
			refs = append(refs, p.Path())
		}
		return p.Name()
	}
	out := make([]shapeField, 0, len(fields))
	for _, f := range fields {
		if f.Name == "_" {
			continue
		}
		out = append(out, shapeField{
			Name:     f.Name,
			Type:     types.TypeString(f.TypeRef, qualify),
			Tag:      string(f.Tag),
			Embedded: f.IsEmbedded,
		})
	}
	return out, refs
}

func sliceElem(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	switch u := t.Underlying().(type) {
	case *types.Slice:
		return u.Elem()
	case *types.Array:
		return u.Elem()
	}
	return nil
}

func isNamedStruct(f shared.FieldInfo) bool {
	named, ok := shared.UnwrapNamed(f.TypeRef)
	if !ok {
		return false
	}
	_, isStruct := named.Underlying().(*types.Struct)
	return isStruct
}

// Tags reachable only through a non-embed field never populate at run time, so
// they are an error rather than a silent no-op.
