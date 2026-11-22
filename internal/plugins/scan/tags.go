package scan

import (
	"fmt"
	"go/types"
	"strings"

	"github.com/ribora/whisk/internal/shared"
)

// scan:"query=tag,repeated" -> scope query, key tag, LookupAll instead of Lookup.
func parseScopedTag(structName, pkgPath string, f shared.FieldInfo, raw string) (sourceField, error) {
	eq := strings.Index(raw, "=")
	if eq < 0 {
		return sourceField{}, fmt.Errorf("%s.%s: scan tag %q must be \"-\", \"embed\", or \"<scope>=<key>[,opt...]\"", structName, f.Name, raw)
	}
	scope := strings.TrimSpace(raw[:eq])
	if scope == "" {
		return sourceField{}, fmt.Errorf("%s.%s: scan tag %q has empty scope", structName, f.Name, raw)
	}
	parts := strings.Split(raw[eq+1:], ",")
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return sourceField{}, fmt.Errorf("%s.%s: scan tag %q has empty key", structName, f.Name, raw)
	}

	sf := sourceField{
		Name:  f.Name,
		Scope: scope,
		Key:   key,
		Type:  f.Type,
		Kind:  basicKind(f.TypeRef),
		Text:  decodesText(f.TypeRef),
	}
	// Slices only. A map and an array report an ElemType as well, and taking it
	// wrote `make([]T, …)` into a field that is neither — code that does not
	// compile. Both fall through to the marker instead.
	if f.IsSlice && !f.IsArray {
		sf.ElemType = f.ElemType
		elem := sliceElem(f.TypeRef)
		sf.ElemKind = basicKind(elem)
		// `var elem *T` then elem.UnmarshalText compiles and dereferences nil, so a
		// pointer element takes the marker instead.
		_, elemIsPointer := elem.(*types.Pointer)
		sf.ElemText = !elemIsPointer && decodesText(elem)
	}
	if sf.Text && f.IsPointer {
		sf.Pointee = strings.TrimPrefix(f.Type, "*")
	}
	for _, opt := range parts[1:] {
		opt = strings.TrimSpace(opt)
		switch opt {
		case "":
			// trailing/extra comma — tolerate
		case "repeated":
			sf.Repeated = true
		default:
			return sourceField{}, fmt.Errorf("%s.%s: unknown scan option %q (allowed: repeated)", structName, f.Name, opt)
		}
	}
	if sf.Repeated && !f.IsSlice {
		return sourceField{}, fmt.Errorf("%s.%s: scan option \"repeated\" requires a slice field, got %s", structName, f.Name, f.Type)
	}
	// After the options: an option is free to change the shape the emitter writes,
	// and the imports follow that shape.
	sf.Refs = spelledOutTypes(sf, f, pkgPath)
	return sf, nil
}
