package validate

import (
	"bytes"
	"fmt"
)

type dispatcher struct {
	handlers map[string]handler
}

func newDispatcher(resolutions map[string]tagResolution) *dispatcher {
	d := &dispatcher{handlers: map[string]handler{}}
	d.handlers["required"] = requiredHandler{}
	d.handlers["unique"] = uniqueHandler{}
	for op := range comparisonOps {
		d.handlers[op] = comparisonHandler{}
	}
	// Constructed last: the closure has to see the populated map.
	dh := diveHandler{}
	dh.generate = func(buf *bytes.Buffer, f field, ctx context) error {
		return d.fieldValidation(buf, f, ctx)
	}
	d.handlers["dive"] = dh

	custom := customHandler{resolve: func(tag string) tagResolution {
		return resolutions[tag]
	}}
	for tag := range resolutions {
		d.handlers[tag] = custom
	}
	return d
}

// required is emitted unguarded; everything else sits inside a zero-value guard
// so it stays quiet on an empty optional field.
func (d *dispatcher) fieldValidation(buf *bytes.Buffer, f field, ctx context) error {
	if f.IsEmbedded {
		// Or s.Embed.Validate() would name a method nobody generated.
		if willHaveValidate(f.TypeRef) {
			return emitEmbedded(buf, f, ctx)
		}
		return nil
	}

	tags := f.ParsedTags
	isRequired := false
	for _, t := range tags {
		if t.Name == "required" {
			isRequired = true
			break
		}
	}

	var inner bytes.Buffer
	hasValidation := false
	for _, t := range tags {
		h, ok := d.handlers[t.Name]
		if !ok {
			return fmt.Errorf("unsupported validator tag: %s", t.Name)
		}
		tagCtx := ctx
		tagCtx.IsRequired = isRequired
		handled, err := h.handle(&inner, f, t, tagCtx)
		if err != nil {
			return err
		}
		if handled {
			hasValidation = true
		}
		// Dive takes the remaining tags itself.
		if t.Name == "dive" {
			break
		}
		// Inside the guard it would never fire on the empty input it exists for.
		if handled && t.Name == "required" {
			buf.Write(inner.Bytes())
			inner.Reset()
		}
	}

	if hasValidation && inner.Len() > 0 {
		wrapValidation(buf, &inner, f, ctx.Prefix)
	}
	return nil
}

// A struct field gets no guard: the dive handler deals with nil and empty
// itself.
func wrapValidation(outer, inner *bytes.Buffer, f field, prefix string) {
	name := prefix + f.Name
	if f.AnonymousType != nil {
		if f.IsPointer {
			guarded(outer, inner, name+" != nil")
			return
		}
		outer.Write(inner.Bytes())
		return
	}
	if f.divesIntoValue() {
		outer.Write(inner.Bytes())
		return
	}
	guard, ok := f.presenceGuard(name)
	if !ok {
		outer.Write(inner.Bytes())
		return
	}
	guarded(outer, inner, guard)
}

func guarded(outer, inner *bytes.Buffer, cond string) {
	fmt.Fprintf(outer, "\tif %s {\n", cond)
	outer.Write(inner.Bytes())
	outer.WriteString("\t}\n")
}

// Joined sub-errors are unwrapped so the path stays flat, matching how Go
// promotes the fields themselves.
func emitEmbedded(buf *bytes.Buffer, f field, ctx context) error {
	name := ctx.Prefix + f.Name
	if f.IsPointer {
		fmt.Fprintf(buf, "\tif %s != nil {\n", name)
		fmt.Fprintf(buf, "\t\tif err := %s.Validate(); err != nil {\n", name)
		buf.WriteString("\t\t\tvalidate.AppendSubErrors(&errs, err)\n")
		buf.WriteString("\t\t}\n\t}\n")
		return nil
	}
	fmt.Fprintf(buf, "\tif err := %s.Validate(); err != nil {\n", name)
	buf.WriteString("\t\tvalidate.AppendSubErrors(&errs, err)\n")
	buf.WriteString("\t}\n")
	return nil
}

func emitValidateMethod(buf *bytes.Buffer, d *dispatcher, name, pkgPath string, hasHook bool, fields []field, extractTags []string) error {
	var fieldBuf bytes.Buffer
	ctx := context{
		Prefix:      "s.",
		ExtractTags: extractTags,
		CurrentPkg:  pkgPath,
	}
	for _, f := range fields {
		if err := d.fieldValidation(&fieldBuf, f, ctx); err != nil {
			return err
		}
	}

	// Pointer receiver, as with Parse: no copy per call.
	fmt.Fprintf(buf, "func (s *%s) Validate() error {\n", name)
	buf.WriteString("\tvar errs []error\n")
	buf.Write(fieldBuf.Bytes())
	if hasHook {
		buf.WriteString("\tif e := s.validate(); e != nil {\n")
		buf.WriteString("\t\tvalidate.AppendSubErrors(&errs, e)\n")
		buf.WriteString("\t}\n")
	}
	buf.WriteString("\treturn errors.Join(errs...)\n}\n\n")
	return nil
}
