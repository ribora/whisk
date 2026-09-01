package validate

import (
	"bytes"
	"fmt"
)

// Prefix is what a field name is written after: "s." at the top, "v." inside a
// dive.
type context struct {
	StoragePath string
	Prefix      string
	CaptureErr  bool
	IsRequired  bool
	ExtractTags []string
	CurrentPkg  string
}

// forElement is the context the body of a dive runs in: the same settings, but names
// are relative to the element and errors are collected rather than returned.
func (c context) forElement(prefix string) context {
	return context{
		StoragePath: c.StoragePath,
		Prefix:      prefix,
		CaptureErr:  true,
		ExtractTags: c.ExtractTags,
		CurrentPkg:  c.CurrentPkg,
	}
}

type tagResolution struct {
	Prefix       string // "validate" (built-in) or "checks" (user)
	ReturnsError bool
	Supported    bool
}

// false means the tag is somebody else's; an error means it was misused.
type handler interface {
	handle(buf *bytes.Buffer, f field, t Tag, ctx context) (bool, error)
}

// errCapture names an already-bound error to carry into Err, or "".
func emitValidationError(buf *bytes.Buffer, checkExpr, seg, tagName, tagValue, kind, errCapture string) {
	fmt.Fprintf(buf, "\tif %s {\n", checkExpr)
	buf.WriteString("\t\terrs = append(errs, &validate.ValidationError{\n")
	if seg != "" {
		fmt.Fprintf(buf, "\t\t\tSegments: validate.Path{%s},\n", seg)
	}
	fmt.Fprintf(buf, "\t\t\tTag:   %q,\n", tagName)
	if tagValue != "" {
		fmt.Fprintf(buf, "\t\t\tValue: %q,\n", tagValue)
	}
	fmt.Fprintf(buf, "\t\t\tKind:  %s,\n", kind)
	if errCapture != "" {
		fmt.Fprintf(buf, "\t\t\tErr:   %s,\n", errCapture)
	}
	buf.WriteString("\t\t})\n\t}\n")
}
