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
	Name  string
	IsPtr bool
}

type sourceField struct {
	Name     string
	Scope    string
	Key      string
	Type     string // Go type as text — drives strconv.ParseInt/ParseBool/... selection
	ElemType string // populated for []T fields
	Repeated bool   // true ⇒ emit src.LookupAll, false ⇒ comma-split src.Lookup
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
			lex.HasBodyFields = true
		case "-":
			// Empty: handled by body unmarshal (or ignored). Dash: skip.
		case "embed":
			// embed delegates to a single struct's Parse, so slice/map fields
			// have no single target to delegate to.
			if f.IsSlice || f.IsMap {
				return lexem{}, nil, fmt.Errorf("%s.%s: scan:\"embed\" cannot be used on a slice/map field (%s) — use a parse() hook for per-element binding", s.Name, f.Name, f.Type)
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
			lex.Delegates = append(lex.Delegates, delegate{Name: f.Name, IsPtr: f.IsPointer})
		default:
			sf, err := parseScopedTag(s.Name, f, raw)
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
func parseScopedTag(structName string, f shared.FieldInfo, raw string) (sourceField, error) {
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
		ElemType: f.ElemType,
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
	return sf, nil
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
