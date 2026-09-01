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

	// Only when some field is untagged: otherwise the unmarshal has nowhere to
	// write and can only fail on a body shape that does not match. It runs first
	// so that path and query stay authoritative over the body.
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
	if f.Text {
		emitTextUnmarshal(sb, "v", f, "\t\t")
	} else {
		emitDecode(sb, "v", target{dst: "s." + f.Name, typ: f.Type, name: f.Name}, f.Kind, "\t\t")
	}
	sb.WriteString("\t}\n")
}

// s is a pointer, so a value field is addressable and needs no temporary.
func emitTextUnmarshal(sb *strings.Builder, input string, f sourceField, ind string) {
	if f.Pointee != "" {
		fmt.Fprintf(sb, "%sif s.%s == nil {\n", ind, f.Name)
		fmt.Fprintf(sb, "%s\ts.%s = new(%s)\n", ind, f.Name, f.Pointee)
		fmt.Fprintf(sb, "%s}\n", ind)
	}
	fmt.Fprintf(sb, "%sif err := s.%s.UnmarshalText([]byte(%s)); err != nil {\n", ind, f.Name, input)
	fmt.Fprintf(sb, "%s\treturn failure.Wrap(\"%%w: %s: %%w\", scan.ErrPayload, err)\n", ind, f.Name)
	fmt.Fprintf(sb, "%s}\n", ind)
}

func emitTextAppend(sb *strings.Builder, input string, f sourceField, ind string) {
	fmt.Fprintf(sb, "%svar elem %s\n", ind, f.ElemType)
	fmt.Fprintf(sb, "%sif err := elem.UnmarshalText([]byte(%s)); err != nil {\n", ind, input)
	fmt.Fprintf(sb, "%s\treturn failure.Wrap(\"%%w: %s: %%w\", scan.ErrPayload, err)\n", ind, f.Name)
	fmt.Fprintf(sb, "%s}\n", ind)
	fmt.Fprintf(sb, "%ss.%s = append(s.%s, elem)\n", ind, f.Name, f.Name)
}

func renderSliceField(sb *strings.Builder, f sourceField) {
	if f.Repeated {
		fmt.Fprintf(sb, "\tif vs := src.LookupAll(%q, %q); len(vs) > 0 {\n", f.Scope, f.Key)
		fmt.Fprintf(sb, "\t\ts.%s = make([]%s, 0, len(vs))\n", f.Name, f.ElemType)
		sb.WriteString("\t\tfor _, v := range vs {\n\t\t\tv = strings.TrimSpace(v)\n")
		if f.ElemText {
			emitTextAppend(sb, "v", f, "\t\t\t")
		} else {
			emitDecode(sb, "v", target{dst: "s." + f.Name, typ: f.ElemType, name: f.Name, elem: true}, f.ElemKind, "\t\t\t")
		}
		sb.WriteString("\t\t}\n\t}\n")
		return
	}

	// Empty parts are skipped, or a trailing comma would add a zero value.
	fmt.Fprintf(sb, "\tif v := src.Lookup(%q, %q); v != \"\" {\n", f.Scope, f.Key)
	sb.WriteString("\t\tparts := strings.Split(v, \",\")\n")
	fmt.Fprintf(sb, "\t\ts.%s = make([]%s, 0, len(parts))\n", f.Name, f.ElemType)
	sb.WriteString("\t\tfor _, p := range parts {\n\t\t\tp = strings.TrimSpace(p)\n")
	sb.WriteString("\t\t\tif p == \"\" {\n\t\t\t\tcontinue\n\t\t\t}\n")
	if f.ElemText {
		emitTextAppend(sb, "p", f, "\t\t\t")
	} else {
		emitDecode(sb, "p", target{dst: "s." + f.Name, typ: f.ElemType, name: f.Name, elem: true}, f.ElemKind, "\t\t\t")
	}
	sb.WriteString("\t\t}\n\t}\n")
}
