package validate

import (
	"bytes"
	"fmt"
)

// dispatcher routes one (field, tag) pair to the matching handler. The
// set of registered handlers is built once per Generate call from the
// observed tag set.
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
	// Dive handler closes over the dispatcher for recursion into element
	// validations. Constructed last so the closure sees the populated map.
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

// fieldValidation emits the validation for one field, walking through its
// parsed tags. Required is emitted at the top level (unwrapped); other
// tags are wrapped in a zero-value guard so they don't fire on empty
// optional fields.
func (d *dispatcher) fieldValidation(buf *bytes.Buffer, f field, ctx context) error {
	if f.IsEmbedded {
		// Delegate only if the embed actually has a Validate; otherwise
		// emitting s.Embed.Validate() would reference a method that was
		// never generated.
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
		// Dive consumes the remaining tags itself (element-level tags),
		// so we stop iterating once we've handed off to it.
		if t.Name == "dive" {
			break
		}
		// Required must live outside the zero-value guard — otherwise it
		// would never fire on empty inputs. Flush immediately and reset.
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

// wrapValidation wraps the inner block in the appropriate zero-value
// guard so optional empty fields don't trigger the per-tag checks. Struct
// fields (anon or auto-dive) emit no guard — the dive handler manages
// nil/empty internally.
func wrapValidation(outer, inner *bytes.Buffer, f field, prefix string) {
	if f.AnonymousType != nil && f.IsPointer {
		fmt.Fprintf(outer, "\tif %s%s != nil {\n", prefix, f.Name)
		outer.Write(inner.Bytes())
		outer.WriteString("\t}\n")
		return
	}
	if f.IsStruct || f.AnonymousType != nil {
		outer.Write(inner.Bytes())
		return
	}
	name := prefix + f.Name
	switch {
	case f.IsPointer:
		fmt.Fprintf(outer, "\tif %s != nil {\n", name)
		outer.Write(inner.Bytes())
		outer.WriteString("\t}\n")
	case f.IsSlice || f.IsMap:
		fmt.Fprintf(outer, "\tif len(%s) > 0 {\n", name)
		outer.Write(inner.Bytes())
		outer.WriteString("\t}\n")
	case f.Type == "string":
		fmt.Fprintf(outer, "\tif %s != \"\" {\n", name)
		outer.Write(inner.Bytes())
		outer.WriteString("\t}\n")
	case f.Type == "bool":
		outer.Write(inner.Bytes())
	default:
		fmt.Fprintf(outer, "\tif %s != 0 {\n", name)
		outer.Write(inner.Bytes())
		outer.WriteString("\t}\n")
	}
}

// emitEmbedded delegates to the embed's Validate, folding any returned
// errors into the outer's errs slice. Errors.Join'd sub-errors are
// unwrapped so path segments stay flat — matching Go's field-promotion
// semantics.
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

// emitValidateMethod writes one struct's Validate method into buf.
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

	// Pointer receiver, unified with the scan plugin's Parse and avoiding a
	// copy of the struct on every Validate call.
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
