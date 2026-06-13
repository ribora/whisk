package scan

import (
	"fmt"
	"strings"
)

func render(lexems []lexem, format Format) []byte {
	var sb strings.Builder
	for _, l := range lexems {
		renderOne(&sb, l, format)
	}
	return []byte(sb.String())
}

func renderOne(sb *strings.Builder, l lexem, format Format) {
	fmt.Fprintf(sb, "func (s *%s) Parse(src scan.Source) error {\n", l.Name)

	// Body unmarshal is unconditional per whisk's tag-isolation design:
	// even tagless structs decode cleanly via json.Unmarshal (field-name
	// matching). Everything that follows — embed delegates and scoped
	// lookups — runs AFTER the body so structured sources stay
	// authoritative: a request body must not override a value an embed
	// populated from a non-body scope (context/path/query).
	if l.HasBodyFields {
		sb.WriteString("\tif body := src.Body(); len(body) > 0 {\n")
		fmt.Fprintf(sb, "\t\tif err := %s(body, s); err != nil {\n", format.Unmarshal)
		sb.WriteString("\t\t\treturn failure.Wrap(\"%w: %w\", scan.ErrPayload, err)\n")
		sb.WriteString("\t\t}\n\t}\n")
	}

	for _, d := range l.Delegates {
		renderDelegate(sb, d)
	}

	for _, f := range l.Fields {
		renderField(sb, f)
	}

	if l.HasHook {
		sb.WriteString("\tif err := s.parse(src); err != nil {\n\t\treturn err\n\t}\n")
	}
	sb.WriteString("\treturn nil\n}\n\n")
}

func renderDelegate(sb *strings.Builder, d delegate) {
	if d.IsPtr {
		fmt.Fprintf(sb, "\tif s.%s != nil {\n", d.Name)
		fmt.Fprintf(sb, "\t\tif err := s.%s.Parse(src); err != nil {\n\t\t\treturn err\n\t\t}\n", d.Name)
		sb.WriteString("\t}\n")
		return
	}
	fmt.Fprintf(sb, "\tif err := s.%s.Parse(src); err != nil {\n\t\treturn err\n\t}\n", d.Name)
}

func renderField(sb *strings.Builder, f sourceField) {
	if f.ElemType != "" {
		renderSliceField(sb, f)
		return
	}
	renderScalarField(sb, f)
}

func renderScalarField(sb *strings.Builder, f sourceField) {
	fmt.Fprintf(sb, "\tif v := src.Lookup(%q, %q); v != \"\" {\n", f.Scope, f.Key)
	emitScalarConvert(sb, "v", f.Type, "s."+f.Name, f.Name, "\t\t")
	sb.WriteString("\t}\n")
}

func renderSliceField(sb *strings.Builder, f sourceField) {
	if f.Repeated {
		fmt.Fprintf(sb, "\tif vs := src.LookupAll(%q, %q); len(vs) > 0 {\n", f.Scope, f.Key)
		fmt.Fprintf(sb, "\t\ts.%s = make([]%s, 0, len(vs))\n", f.Name, f.ElemType)
		sb.WriteString("\t\tfor _, v := range vs {\n\t\t\tv = strings.TrimSpace(v)\n")
		emitSliceAppend(sb, "v", f.ElemType, "s."+f.Name, f.Name, "\t\t\t")
		sb.WriteString("\t\t}\n\t}\n")
		return
	}

	// Non-repeated slice: comma-split a single Lookup value. Skip empty
	// parts so trailing/leading commas don't produce zero-value entries.
	fmt.Fprintf(sb, "\tif v := src.Lookup(%q, %q); v != \"\" {\n", f.Scope, f.Key)
	sb.WriteString("\t\tparts := strings.Split(v, \",\")\n")
	fmt.Fprintf(sb, "\t\ts.%s = make([]%s, 0, len(parts))\n", f.Name, f.ElemType)
	sb.WriteString("\t\tfor _, p := range parts {\n\t\t\tp = strings.TrimSpace(p)\n")
	sb.WriteString("\t\t\tif p == \"\" {\n\t\t\t\tcontinue\n\t\t\t}\n")
	emitSliceAppend(sb, "p", f.ElemType, "s."+f.Name, f.Name, "\t\t\t")
	sb.WriteString("\t\t}\n\t}\n")
}

func emitScalarConvert(sb *strings.Builder, input, typ, dst, name, ind string) {
	switch typ {
	case "string":
		fmt.Fprintf(sb, "%s%s = %s\n", ind, dst, input)
	case "int", "int8", "int16", "int32", "int64":
		fmt.Fprintf(sb, "%sparsed, err := strconv.ParseInt(%s, 10, %d)\n", ind, input, bitSize(typ))
		emitWrapErr(sb, name, ind)
		fmt.Fprintf(sb, "%s%s = %s(parsed)\n", ind, dst, typ)
	case "uint", "uint8", "uint16", "uint32", "uint64":
		fmt.Fprintf(sb, "%sparsed, err := strconv.ParseUint(%s, 10, %d)\n", ind, input, bitSize(typ))
		emitWrapErr(sb, name, ind)
		fmt.Fprintf(sb, "%s%s = %s(parsed)\n", ind, dst, typ)
	case "float32", "float64":
		fmt.Fprintf(sb, "%sparsed, err := strconv.ParseFloat(%s, %d)\n", ind, input, bitSize(typ))
		emitWrapErr(sb, name, ind)
		fmt.Fprintf(sb, "%s%s = %s(parsed)\n", ind, dst, typ)
	case "bool":
		fmt.Fprintf(sb, "%sparsed, err := strconv.ParseBool(%s)\n", ind, input)
		emitWrapErr(sb, name, ind)
		fmt.Fprintf(sb, "%s%s = parsed\n", ind, dst)
	default:
		fmt.Fprintf(sb, "%s// TODO whisk: cannot convert string to %s for %s — handle in parse(src) hook\n", ind, typ, name)
		fmt.Fprintf(sb, "%s_ = %s\n", ind, input)
	}
}

func emitSliceAppend(sb *strings.Builder, input, elem, dst, name, ind string) {
	switch elem {
	case "string":
		fmt.Fprintf(sb, "%s%s = append(%s, %s)\n", ind, dst, dst, input)
	case "int", "int8", "int16", "int32", "int64":
		fmt.Fprintf(sb, "%sparsed, err := strconv.ParseInt(%s, 10, %d)\n", ind, input, bitSize(elem))
		emitWrapErr(sb, name, ind)
		fmt.Fprintf(sb, "%s%s = append(%s, %s(parsed))\n", ind, dst, dst, elem)
	case "uint", "uint8", "uint16", "uint32", "uint64":
		fmt.Fprintf(sb, "%sparsed, err := strconv.ParseUint(%s, 10, %d)\n", ind, input, bitSize(elem))
		emitWrapErr(sb, name, ind)
		fmt.Fprintf(sb, "%s%s = append(%s, %s(parsed))\n", ind, dst, dst, elem)
	case "float32", "float64":
		fmt.Fprintf(sb, "%sparsed, err := strconv.ParseFloat(%s, %d)\n", ind, input, bitSize(elem))
		emitWrapErr(sb, name, ind)
		fmt.Fprintf(sb, "%s%s = append(%s, %s(parsed))\n", ind, dst, dst, elem)
	case "bool":
		fmt.Fprintf(sb, "%sparsed, err := strconv.ParseBool(%s)\n", ind, input)
		emitWrapErr(sb, name, ind)
		fmt.Fprintf(sb, "%s%s = append(%s, parsed)\n", ind, dst, dst)
	default:
		fmt.Fprintf(sb, "%s// TODO whisk: cannot convert string to %s for %s element — handle in parse(src) hook\n", ind, elem, name)
		fmt.Fprintf(sb, "%s_ = %s\n", ind, input)
	}
}

func emitWrapErr(sb *strings.Builder, name, ind string) {
	fmt.Fprintf(sb, "%sif err != nil {\n", ind)
	fmt.Fprintf(sb, "%s\treturn failure.Wrap(\"%%w: %s: %%w\", scan.ErrPayload, err)\n", ind, name)
	fmt.Fprintf(sb, "%s}\n", ind)
}

// bitSize returns the strconv bit-size argument for typ. `int`/`uint` are
// parsed as 64-bit so values fit on 64-bit hosts — the cast back narrows
// to the platform width.
func bitSize(typ string) int {
	switch typ {
	case "int8", "uint8":
		return 8
	case "int16", "uint16":
		return 16
	case "int32", "uint32", "float32":
		return 32
	}
	return 64
}

// neededImports inspects the lexem set to decide which import paths the
// generated section references. Body unmarshal is always emitted, so the
// body-format import and failure are always pulled in.
func neededImports(lexems []lexem, format Format) []string {
	wantStrconv := false
	wantStrings := false
	wantBody := false
	for _, l := range lexems {
		wantBody = wantBody || l.HasBodyFields
		for _, f := range l.Fields {
			if f.ElemType != "" {
				wantStrings = true
				if needsStrconv(f.ElemType) {
					wantStrconv = true
				}
				continue
			}
			if needsStrconv(f.Type) {
				wantStrconv = true
			}
		}
	}
	out := []string{"gitlab.com/gorib/failure", "gitlab.com/gorib/whisk/scan"}
	if wantBody {
		out = append(out, format.Import)
	}
	if wantStrconv {
		out = append(out, "strconv")
	}
	if wantStrings {
		out = append(out, "strings")
	}
	return out
}

func needsStrconv(typ string) bool {
	switch typ {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"float32", "float64", "bool":
		return true
	}
	return false
}
