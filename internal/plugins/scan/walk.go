package scan

import (
	"fmt"
	"strings"

	"gitlab.com/gorib/whisk/internal/shared"
)

func walk(s shared.StructInfo, formatTag string) (lexem, []string, error) {
	if err := checkBordering(s); err != nil {
		return lexem{}, nil, err
	}

	lex := lexem{Name: s.Name, HasHook: s.HasParseHook}
	var warnings []string
	for _, f := range s.Fields {
		raw := strings.TrimSpace(f.Tag.Get("scan"))
		switch raw {
		case "":
			// An untagged field is what makes a body unmarshal necessary.
			lex.HasBodyFields = true
		case "-":
			// Opts out of every source.
		case "embed":
			// embed delegates to one Parse, and a collection has no single target.
			if f.IsSlice || f.IsMap {
				return lexem{}, nil, fmt.Errorf("%s.%s: scan:\"embed\" cannot be used on a slice/map field (%s) — use a parse() hook for per-element binding", s.Name, f.Name, f.Type)
			}
			// A pointer embed used to be allocated as a side effect of the parent's
			// body unmarshal, which broke once the body became conditional.
			if f.IsPointer {
				return lexem{}, nil, fmt.Errorf("%s.%s: scan:\"embed\" on a pointer field (%s) is not supported — use a value embed, or a parse() hook with explicit allocation", s.Name, f.Name, f.Type)
			}
			// An inline struct carries no Parse to delegate to.
			if !isNamedStruct(f) {
				return lexem{}, nil, fmt.Errorf("%s.%s: scan:\"embed\" requires a named struct type (so it has a Parse method to delegate to), got %s", s.Name, f.Name, f.Type)
			}
			// embed reads the inner fields from the same body; a format name makes
			// the field a nested object. Both were honoured, and the flat read won.
			if name, _, _ := strings.Cut(f.Tag.Get(formatTag), ","); name != "" && name != "-" {
				return lexem{}, nil, fmt.Errorf("%s.%s: scan:\"embed\" and %s:%q contradict each other — embed reads the inner fields from the same body, a %s name makes this field a nested object; drop the name, or use %s:\"-\" to keep the field out of the body",
					s.Name, f.Name, formatTag, name, formatTag, formatTag)
			}
			// Works, but the embedded Parse promotes into the parent's method set.
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
