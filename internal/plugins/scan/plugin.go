package scan

import (
	"fmt"
	"go/types"
	"reflect"
	"strings"

	"gitlab.com/gorib/whisk/internal/generator"
	"gitlab.com/gorib/whisk/internal/shared"
)

// Plugin generates Parse(src scan.Source) error for every struct.
type Plugin struct {
	Format Format
}

func New(format Format) *Plugin {
	if format.Tag == "" {
		format = FormatJSON
	}
	return &Plugin{Format: format}
}

func (p *Plugin) Name() string { return "scan" }

func (p *Plugin) Generate(structs []shared.StructInfo) (generator.Section, error) {
	lexems := make([]lexem, 0, len(structs))
	var warnings []string
	for _, s := range structs {
		lex, warns, err := walk(s)
		if err != nil {
			return generator.Section{}, err
		}
		warnings = append(warnings, warns...)
		lexems = append(lexems, lex)
	}
	if len(lexems) == 0 {
		return generator.Section{}, nil
	}
	code := render(lexems, p.Format)
	return generator.Section{Code: code, Imports: neededImports(lexems, p.Format), Warnings: warnings}, nil
}

type lexem struct {
	Name          string
	Delegates     []delegate
	Fields        []sourceField
	HasHook       bool // *T defines parse(scan.Source) error
	HasBodyFields bool
}

type delegate struct {
	Name string
}

type sourceField struct {
	Name  string
	Scope string
	Key   string
	// Type is the declared type as text and goes into the conversion the
	// generated code performs; Kind is its underlying basic type and is what
	// selects strconv.ParseInt/ParseBool/... So `type Page int` converts with
	// strconv.ParseInt and lands as Page(parsed).
	Type     string
	Kind     string
	ElemType string // populated for []T fields
	ElemKind string
	Repeated bool // true ⇒ emit src.LookupAll, false ⇒ comma-split src.Lookup
	// Text is set when the type decodes itself from text. It wins over Kind:
	// a type that declares UnmarshalText means to be built through it, and
	// converting around it would silently drop whatever it normalises.
	// Pointee is what a pointer field allocates before the call.
	Text     bool
	Pointee  string
	ElemText bool
	// Refs are import paths of types the emitted code spells out — a make, a
	// new, or a conversion. Only those: an import Go cannot see used is a
	// compile error, so a superset here is as broken as a missing one.
	Refs []string
}

func walk(s shared.StructInfo) (lexem, []string, error) {
	if err := checkBordering(s); err != nil {
		return lexem{}, nil, err
	}

	lex := lexem{Name: s.Name, HasHook: s.HasParseHook}
	var warnings []string
	for _, f := range s.Fields {
		raw := strings.TrimSpace(f.Tag.Get("scan"))
		switch raw {
		case "":
			// No scan tag — this field is the reason a body unmarshal must
			// be emitted (json.Unmarshal field-name match into s).
			lex.HasBodyFields = true
		case "-":
			// Field opts out of every source; codegen ignores it.
		case "embed":
			// embed delegates to a single struct's Parse, so slice/map fields
			// have no single target to delegate to.
			if f.IsSlice || f.IsMap {
				return lexem{}, nil, fmt.Errorf("%s.%s: scan:\"embed\" cannot be used on a slice/map field (%s) — use a parse() hook for per-element binding", s.Name, f.Name, f.Type)
			}
			// Pointer embeds used to be allocated as a side effect of the
			// parent's body unmarshal — fragile and broken once body
			// becomes conditional. Force value embeds; the parse() hook is
			// the supported escape hatch for conditional allocation.
			if f.IsPointer {
				return lexem{}, nil, fmt.Errorf("%s.%s: scan:\"embed\" on a pointer field (%s) is not supported — use a value embed, or a parse() hook with explicit allocation", s.Name, f.Name, f.Type)
			}
			// Must be a named struct: an inline anonymous struct can't carry a
			// Parse method to delegate to.
			if !isNamedStruct(f) {
				return lexem{}, nil, fmt.Errorf("%s.%s: scan:\"embed\" requires a named struct type (so it has a Parse method to delegate to), got %s", s.Name, f.Name, f.Type)
			}
			// Anonymous embedding promotes the embedded Parse into the parent's
			// method set — works, but leaks; warn and prefer a named field.
			if f.IsEmbedded {
				warnings = append(warnings, fmt.Sprintf("%s: embedded field %s uses scan:\"embed\" — the embedded type's generated Parse promotes into %s's method set; prefer a named field", s.Name, f.Name, s.Name))
			}
			lex.Delegates = append(lex.Delegates, delegate{Name: f.Name})
		default:
			sf, err := parseScopedTag(s.Name, s.PkgPath, f, raw)
			if err != nil {
				return lexem{}, nil, err
			}
			lex.Fields = append(lex.Fields, sf)
		}
	}
	return lex, warnings, nil
}

// parseScopedTag parses `<scope>=<key>[,opt1[,opt2...]]`. The only option
// currently recognised is `repeated` (slice fields → src.LookupAll).
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
		Name:     f.Name,
		Scope:    scope,
		Key:      key,
		Type:     f.Type,
		Kind:     basicKind(f.TypeRef),
		ElemType: f.ElemType,
		Text:     decodesText(f.TypeRef),
	}
	if f.ElemType != "" {
		elem := sliceElem(f.TypeRef)
		sf.ElemKind = basicKind(elem)
		// A pointer element would need allocating per iteration; `var elem *T`
		// followed by elem.UnmarshalText compiles and then dereferences nil, so
		// pointer elements fall through to the marker instead.
		_, elemIsPointer := elem.(*types.Pointer)
		sf.ElemText = !elemIsPointer && decodesText(elem)
	}
	if sf.Text && f.IsPointer {
		sf.Pointee = strings.TrimPrefix(f.Type, "*")
	}
	sf.Refs = spelledOutTypes(sf, f, pkgPath)
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
	return sf, nil
}

// basicKind names t's underlying basic type — "string", "int", "bool" — which
// is what a strconv call is chosen by. Empty when the underlying type is not
// basic, and then the field falls through to the TODO marker as before.
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

// spelledOutTypes lists the packages of types the emitter writes literally:
// the element type in a make and a var, the pointee in a new, and the declared
// type in a conversion. A scalar decoded through UnmarshalText names nothing —
// the call goes through the field — so it contributes no import.
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

// decodesText reports whether a value of t can be built from a text scope by
// calling UnmarshalText on it. The method is virtually always on the pointer
// receiver, so a value field is asked about *T; a pointer field is asked about
// itself and allocates before the call.
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

// checkBordering errors when a field's type carries scan tags reachable only
// through a non-embed field — those tags would never populate at runtime.
// Recursion follows scan:"embed" only, matching how generated Parse chains.
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
