package scan

import (
	"fmt"
	"strings"
)

// A decoded value either replaces the field or is appended to it. Everything before
// that — which strconv call, which bit size, which conversion — is the same, and
// writing it out twice is how the two copies came to disagree about their markers.
type target struct {
	dst  string
	typ  string
	name string
	elem bool
}

func (t target) put(expr string) string {
	if t.elem {
		return fmt.Sprintf("%s = append(%s, %s)", t.dst, t.dst, expr)
	}
	return fmt.Sprintf("%s = %s", t.dst, expr)
}

func (t target) marker() string {
	subject := t.name
	if t.elem {
		subject += " element"
	}
	return fmt.Sprintf("// TODO whisk: cannot convert string to %s for %s — handle in parse(src) hook", t.typ, subject)
}

// parsed is converted unconditionally for the numeric kinds: strconv returns the
// widest type, so `int(parsed)` is needed and `int64(parsed)` is harmless.
func emitDecode(sb *strings.Builder, input string, t target, kind, ind string) {
	parse := func(call string, args ...any) {
		fmt.Fprintf(sb, ind+call+"\n", args...)
		emitWrapErr(sb, t.name, ind)
		fmt.Fprintf(sb, "%s%s\n", ind, t.put(t.typ+"(parsed)"))
	}

	switch kind {
	case "string":
		fmt.Fprintf(sb, "%s%s\n", ind, t.put(convert(t.typ, kind, input)))
	case "int", "int8", "int16", "int32", "int64":
		parse("parsed, err := strconv.ParseInt(%s, 10, %d)", input, bitSize(t.typ))
	case "uint", "uint8", "uint16", "uint32", "uint64":
		parse("parsed, err := strconv.ParseUint(%s, 10, %d)", input, bitSize(t.typ))
	case "float32", "float64":
		parse("parsed, err := strconv.ParseFloat(%s, %d)", input, bitSize(t.typ))
	case "bool":
		fmt.Fprintf(sb, "%sparsed, err := strconv.ParseBool(%s)\n", ind, input)
		emitWrapErr(sb, t.name, ind)
		fmt.Fprintf(sb, "%s%s\n", ind, t.put(convert(t.typ, kind, "parsed")))
	default:
		fmt.Fprintf(sb, "%s%s\n", ind, t.marker())
		fmt.Fprintf(sb, "%s_ = %s\n", ind, input)
	}
}

// Status(v) for a named type, bare for a plain one.
func convert(typ, kind, expr string) string {
	if typ == kind {
		return expr
	}
	return typ + "(" + expr + ")"
}

func emitWrapErr(sb *strings.Builder, name, ind string) {
	fmt.Fprintf(sb, "%sif err != nil {\n", ind)
	fmt.Fprintf(sb, "%s\treturn failure.Wrap(\"%%w: %s: %%w\", scan.ErrPayload, err)\n", ind, name)
	fmt.Fprintf(sb, "%s}\n", ind)
}

// int and uint parse as 64-bit so a value fits on a 64-bit host; the cast back
// narrows to the platform width.
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

// failure is needed only where errors are wrapped — a body unmarshal or a
// strconv conversion.
