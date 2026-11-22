package validate

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ribora/whisk/internal/shared"
)

// The generator is held as a deferred reference: dive recurses into it, and it is
// still being assembled when this is constructed.
type diveHandler struct {
	generate func(buf *bytes.Buffer, f field, ctx context) error
}

func (h diveHandler) handle(buf *bytes.Buffer, f field, _ Tag, ctx context) (bool, error) {
	switch {
	case f.IsSlice:
		return h.overElements(buf, f, ctx, overSlice)
	case f.IsMap:
		return h.overElements(buf, f, ctx, overMap)
	default:
		return h.intoStruct(buf, f, ctx)
	}
}

// The rules written after the dive marker belong to the elements.
func elementTags(f field) []Tag {
	var out []Tag
	found := false
	for _, t := range f.ParsedTags {
		if found {
			out = append(out, t)
		} else if t.Name == "dive" {
			found = true
		}
	}
	return out
}

// The four cases are the same for a slice and a map; only the name an element
// gets in the error path differs.
func (h diveHandler) overElements(buf *bytes.Buffer, f field, ctx context, shape diveShape) (bool, error) {
	tags := elementTags(f)
	access := ctx.Prefix + f.Name
	seg := fieldSegmentCode(f, ctx.ExtractTags)

	// Synthesize the inner container so the next `,dive` routes back here.
	if inner, ok := strings.CutPrefix(f.ElemType, "[]"); ok && len(tags) > 0 {
		return h.eachElement(buf, ctx, shape, access, seg, field{
			FieldInfo: shared.FieldInfo{
				Type: f.ElemType, IsSlice: true, ElemType: inner, TypeRef: diveElem(f),
			},
			ParsedTags: tags,
		})
	}

	// A scalar element, named or not: the rules after the dive are its own, and it
	// is treated exactly like a field carrying them — an empty one is absent, so
	// write `dive,required,min=2` to refuse it. An element carrying its own
	// Validate is dived into instead, below; the two together would need the
	// element promoted like a field, which nothing asks for yet.
	if elem := diveElem(f); len(tags) > 0 && basicUnder(elem) != "" && !willHaveValidate(elem) {
		return h.eachElement(buf, ctx, shape, access, seg, field{
			FieldInfo:  shared.FieldInfo{Type: f.ElemType, TypeRef: elem},
			ParsedTags: tags,
		})
	}

	if f.AnonymousElem != nil {
		return h.eachInlineField(buf, ctx, shape, access, seg, f)
	}

	switch elem := diveElem(f); {
	case isInterfaceType(elem):
		emitDiveAssertion(buf, shape, access, seg)
		return true, nil
	case willHaveValidate(elem):
		emitDiveDirect(buf, shape, access, seg)
		return true, nil
	default:
		return false, nil
	}
}

// No loop unless the element's rules produced something.
func (h diveHandler) eachElement(buf *bytes.Buffer, ctx context, shape diveShape, access, seg string, elem field) (bool, error) {
	var body bytes.Buffer
	if err := h.generate(&body, elem, ctx.forElement("v")); err != nil {
		return false, err
	}
	if body.Len() == 0 {
		return false, nil
	}
	emitDivePrimitive(buf, shape, access, seg, body.String())
	return true, nil
}

// An inline struct has no type of its own to recurse into, so its fields are
// walked here.
func (h diveHandler) eachInlineField(buf *bytes.Buffer, ctx context, shape diveShape, access, seg string, f field) (bool, error) {
	var body bytes.Buffer
	for _, sub := range underlyingStructFields(f.AnonymousElem, ctx.CurrentPkg) {
		if err := h.generate(&body, sub, ctx.forElement("v.")); err != nil {
			return false, err
		}
	}
	if body.Len() == 0 {
		return false, nil
	}
	emitDivePrimitive(buf, shape, access, seg, body.String())
	return true, nil
}

func (h diveHandler) intoStruct(buf *bytes.Buffer, f field, ctx context) (bool, error) {
	name := ctx.Prefix + f.Name
	seg := fieldSegmentCode(f, ctx.ExtractTags)

	if f.AnonymousType != nil {
		var body bytes.Buffer
		for _, sub := range underlyingStructFields(f.AnonymousType, ctx.CurrentPkg) {
			if err := h.generate(&body, sub, ctx.forElement(name+".")); err != nil {
				return false, err
			}
		}
		if body.Len() == 0 {
			return false, nil
		}
		buf.WriteString("\t{\n\t\t_start := len(errs)\n")
		buf.WriteString(body.String())
		buf.WriteString("\t\tif _start < len(errs) {\n")
		fmt.Fprintf(buf, "\t\t\tvalidate.PrependFrom(errs, _start, %s)\n", seg)
		buf.WriteString("\t\t}\n\t}\n")
		return true, nil
	}

	switch elem := diveElem(f); {
	case isInterfaceType(elem):
		// The static type carries no Validate; the dynamic value may.
		guardNil(buf, name, f.IsPointer, func(b *bytes.Buffer) {
			fmt.Fprintf(b, "\tif vv, ok := any(%s).(interface{ Validate() error }); ok {\n", name)
			b.WriteString("\t\tif subErr := vv.Validate(); subErr != nil {\n")
			fmt.Fprintf(b, "\t\t\t%s\n", appendSubErrors(seg))
			b.WriteString("\t\t}\n\t}\n")
		})
		return true, nil
	case willHaveValidate(elem):
		guardNil(buf, name, f.IsPointer, func(b *bytes.Buffer) {
			fmt.Fprintf(b, "\tif subErr := %s.Validate(); subErr != nil {\n", name)
			fmt.Fprintf(b, "\t\t%s\n", appendSubErrors(seg))
			b.WriteString("\t}\n")
		})
		return true, nil
	default:
		return false, nil
	}
}

func appendSubErrors(seg string) string {
	if seg == "" {
		return "validate.AppendSubErrors(&errs, subErr)"
	}
	return "validate.AppendSubErrors(&errs, subErr, " + seg + ")"
}

// Validate on a nil pointer panics, and an absent optional struct is not itself
// an error. Indentation is gofmt's problem, not this function's.
func guardNil(buf *bytes.Buffer, name string, pointer bool, body func(*bytes.Buffer)) {
	if !pointer {
		body(buf)
		return
	}
	fmt.Fprintf(buf, "\tif %s != nil {\n", name)
	body(buf)
	buf.WriteString("\t}\n")
}

// The locator has to reach the error path in every combination, or the answer to
// "which item" is "items". It used to be written out per combination, and the two
// map-of-struct ones were written without it.
type diveShape struct {
	loop    string
	locator string
}

var (
	overSlice = diveShape{loop: "for i, v := range", locator: "validate.NewIndexSegment(i)"}
	overMap   = diveShape{loop: "for k, v := range", locator: "validate.NewKeySegment(k)"}
)

// The segment goes into a local: it is passed to a variadic call once per element.
func (s diveShape) bind(buf *bytes.Buffer, access, seg string) {
	if seg != "" {
		fmt.Fprintf(buf, "\t_seg := %s\n", seg)
	}
	fmt.Fprintf(buf, "\t%s %s {\n", s.loop, access)
}

func (s diveShape) segments(seg string) string {
	if seg == "" {
		return s.locator
	}
	return "_seg, " + s.locator
}

func emitDivePrimitive(buf *bytes.Buffer, s diveShape, access, seg, subValidations string) {
	s.bind(buf, access, seg)
	buf.WriteString("\t\t_start := len(errs)\n")
	buf.WriteString(subValidations)
	buf.WriteString("\t\tif _start < len(errs) {\n")
	fmt.Fprintf(buf, "\t\t\tvalidate.PrependFrom(errs, _start, %s)\n", s.segments(seg))
	buf.WriteString("\t\t}\n\t}\n")
}

func emitDiveDirect(buf *bytes.Buffer, s diveShape, access, seg string) {
	s.bind(buf, access, seg)
	buf.WriteString("\t\tif subErr := v.Validate(); subErr != nil {\n")
	fmt.Fprintf(buf, "\t\t\tvalidate.AppendSubErrors(&errs, subErr, %s)\n", s.segments(seg))
	buf.WriteString("\t\t}\n\t}\n")
}

func emitDiveAssertion(buf *bytes.Buffer, s diveShape, access, seg string) {
	s.bind(buf, access, seg)
	buf.WriteString("\t\tif vv, ok := any(v).(interface{ Validate() error }); ok {\n")
	buf.WriteString("\t\t\tif subErr := vv.Validate(); subErr != nil {\n")
	fmt.Fprintf(buf, "\t\t\t\tvalidate.AppendSubErrors(&errs, subErr, %s)\n", s.segments(seg))
	buf.WriteString("\t\t\t}\n\t\t}\n\t}\n")
}
