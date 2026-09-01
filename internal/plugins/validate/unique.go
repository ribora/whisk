package validate

import (
	"bytes"
	"fmt"
)

// ---- unique ----

type uniqueHandler struct{}

func (uniqueHandler) handle(buf *bytes.Buffer, f field, t Tag, ctx context) (bool, error) {
	if !f.IsSlice {
		return false, nil
	}
	access := ctx.Prefix + f.Name
	seg := fieldSegmentCode(f, ctx.ExtractTags)

	// unique=FieldName: uniqueness keyed by a struct member of each element.
	if t.HasValue && t.Value != "" {
		if isBasicType(f.ElemType) {
			return false, fmt.Errorf("unique=%s: cannot specify field on slice of non-struct type %q (field %q)", t.Value, f.ElemType, f.Name)
		}
		// For anonymous element structs we can verify the named field
		// exists. For named element types we trust the caller — the Go
		// compiler will catch typos at compile time.
		if f.AnonymousElem != nil {
			fields := underlyingStructFields(f.AnonymousElem, ctx.CurrentPkg)
			found := false
			for _, sf := range fields {
				if sf.Name == t.Value {
					found = true
					break
				}
			}
			if !found {
				return false, fmt.Errorf("unique=%s: anonymous struct in slice %q has no field %q", t.Value, f.Name, t.Value)
			}
		}
		fmt.Fprintf(buf, "\tseen%s%s := make(map[any]bool, len(%s))\n", f.Name, t.Value, access)
		fmt.Fprintf(buf, "\tfor i, v := range %s {\n", access)
		fmt.Fprintf(buf, "\t\tif seen%s%s[v.%s] {\n", f.Name, t.Value, t.Value)
		buf.WriteString("\t\t\terrs = append(errs, &validate.ValidationError{\n")
		if seg != "" {
			fmt.Fprintf(buf, "\t\t\t\tSegments: validate.Path{%s, validate.NewIndexSegment(i)},\n", seg)
		} else {
			buf.WriteString("\t\t\t\tSegments: validate.Path{validate.NewIndexSegment(i)},\n")
		}
		buf.WriteString("\t\t\t\tTag:   \"unique\",\n")
		fmt.Fprintf(buf, "\t\t\t\tValue: %q,\n", t.Value)
		fmt.Fprintf(buf, "\t\t\t\tKind:  %s,\n", kindSlice)
		buf.WriteString("\t\t\t})\n\t\t}\n")
		fmt.Fprintf(buf, "\t\tseen%s%s[v.%s] = true\n", f.Name, t.Value, t.Value)
		buf.WriteString("\t}\n")
		return true, nil
	}

	fmt.Fprintf(buf, "\tseen%s := make(map[%s]bool, len(%s))\n", f.Name, f.ElemType, access)
	fmt.Fprintf(buf, "\tfor _, v := range %s {\n", access)
	fmt.Fprintf(buf, "\t\tif seen%s[v] {\n", f.Name)
	buf.WriteString("\t\t\terrs = append(errs, &validate.ValidationError{\n")
	if seg != "" {
		fmt.Fprintf(buf, "\t\t\t\tSegments: validate.Path{%s},\n", seg)
	}
	buf.WriteString("\t\t\t\tTag:  \"unique\",\n")
	fmt.Fprintf(buf, "\t\t\t\tKind: %s,\n", kindSlice)
	buf.WriteString("\t\t\t})\n\t\t}\n")
	fmt.Fprintf(buf, "\t\tseen%s[v] = true\n", f.Name)
	buf.WriteString("\t}\n")
	return true, nil
}
