package scan

import "gitlab.com/gorib/whisk/internal/generator"

func neededImports(lexems []lexem, format Format) []generator.Import {
	wantStrconv := false
	wantStrings := false
	wantBody := false
	wantWrap := false
	var typeRefs []generator.Import
	seenRef := map[string]bool{}
	for _, l := range lexems {
		wantBody = wantBody || l.HasBodyFields
		for _, f := range l.Fields {
			wantWrap = wantWrap || f.Text || f.ElemText
			for _, ref := range f.Refs {
				if !seenRef[ref] {
					seenRef[ref] = true
					typeRefs = append(typeRefs, generator.Import{Path: ref})
				}
			}
			if f.ElemType != "" {
				wantStrings = true
				if needsStrconv(f.ElemKind) {
					wantStrconv = true
				}
				continue
			}
			if needsStrconv(f.Kind) {
				wantStrconv = true
			}
		}
	}
	out := []generator.Import{{Path: "gitlab.com/gorib/whisk/scan"}}
	if wantBody || wantStrconv || wantWrap {
		out = append(out, generator.Import{Path: "gitlab.com/gorib/failure"})
	}
	if wantBody {
		out = append(out, format.Import)
	}
	if wantStrconv {
		out = append(out, generator.Import{Path: "strconv"})
	}
	if wantStrings {
		out = append(out, generator.Import{Path: "strings"})
	}
	return append(out, typeRefs...)
}

// Takes the underlying kind, never the declared name: asking about "Page" would
// answer no and leave strconv out of the imports.
func needsStrconv(kind string) bool {
	switch kind {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "bool":
		return true
	}
	return false
}
