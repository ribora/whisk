package validate

import (
	"bytes"
	"fmt"
)

// ---- required ----

type requiredHandler struct{}

func (requiredHandler) handle(buf *bytes.Buffer, f field, _ Tag, ctx context) (bool, error) {
	fieldName := ctx.Prefix + f.Name
	cond, ok, err := f.zeroCondition(fieldName)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	kind := requiredKind(f)
	seg := fieldSegmentCode(f, ctx.ExtractTags)
	fmt.Fprintf(buf, "\tif %s {\n", cond)
	buf.WriteString("\t\terrs = append(errs, &validate.ValidationError{\n")
	if seg != "" {
		fmt.Fprintf(buf, "\t\t\tSegments: validate.Path{%s},\n", seg)
	}
	buf.WriteString("\t\t\tTag:  \"required\",\n")
	fmt.Fprintf(buf, "\t\t\tKind: %s,\n", kind)
	buf.WriteString("\t\t})\n\t}\n")
	return true, nil
}

func requiredKind(f field) string {
	switch f.shape() {
	case shapeLen:
		return kindSlice
	case shapeString:
		return kindStr
	case shapeTime:
		return kindTime
	case shapeNone:
		return kindObj
	}
	return kindValue
}
