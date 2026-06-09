package validate

import (
	"bytes"
	"fmt"
	"strings"

	"gitlab.com/gorib/whisk/internal/shared"
)

// context carries the per-call configuration each handler needs: where in
// the source tree the user's custom-validator files live, what prefix to
// emit before field names ("s." at top level, "v." inside dive loops),
// whether to capture err in the generated code, and which tag keys to
// pull names from for path segments.
type context struct {
	StoragePath string
	Prefix      string
	CaptureErr  bool
	IsRequired  bool
	ExtractTags []string
	CurrentPkg  string
}

// comparisonOps maps a comparison tag name to the inverted operator used
// in the generated check ("if FAIL"). E.g. `min=8` → fail when `len(x) < 8`.
var comparisonOps = map[string]string{
	"min": "<", "gte": "<", "max": ">", "lte": ">",
	"gt": "<=", "lt": ">=", "eq": "!=", "equal": "!=",
	"ne": "==", "neq": "==", "len": "!=",
}

// tagResolution names where a tag's Is<Tag> function lives.
type tagResolution struct {
	Prefix       string // "validate" (built-in) or "checks" (user)
	ReturnsError bool
	Supported    bool
}

// handler emits the validation code for one (field, tag) pair into buf.
// Return (true, nil) when it handled the tag, (false, nil) when it
// declines (different handler should try), or (_, err) on tag misuse.
type handler interface {
	handle(buf *bytes.Buffer, f field, t Tag, ctx context) (bool, error)
}

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
	fmt.Fprintf(buf, "\t\t\tKind: %q,\n", kind)
	buf.WriteString("\t\t})\n\t}\n")
	return true, nil
}

func requiredKind(f field) string {
	typ := strings.TrimPrefix(f.Type, "*")
	switch {
	case f.IsSlice:
		return "slice"
	case typ == "string":
		return "str"
	case f.IsStruct || (f.IsPointer && !isBasicType(typ)):
		return "obj"
	}
	return "value"
}

// ---- comparison (min/max/gt/lt/eq/ne/len) ----

type comparisonHandler struct{}

func (comparisonHandler) handle(buf *bytes.Buffer, f field, t Tag, ctx context) (bool, error) {
	cmp, ok := comparisonOps[t.Name]
	if !ok {
		return false, nil
	}

	eff := f.effectiveName(ctx.Prefix)
	var (
		checkExpr string
		kind      string
	)

	isTimeField := f.Type == "time.Time" || f.Type == "*time.Time"
	isTimeString := (f.Type == "string" || f.Type == "*string") && isTimeValue(t.Value)

	switch {
	case isTimeField || isTimeString:
		kind = "time"
		switch t.Name {
		case "min", "gte":
			checkExpr = fmt.Sprintf("!validate.IsTimeMin(%s, %q)", eff, t.Value)
		case "max", "lte":
			checkExpr = fmt.Sprintf("!validate.IsTimeMax(%s, %q)", eff, t.Value)
		case "gt":
			checkExpr = fmt.Sprintf("!validate.IsTimeGT(%s, %q)", eff, t.Value)
		case "lt":
			checkExpr = fmt.Sprintf("!validate.IsTimeLT(%s, %q)", eff, t.Value)
		default:
			if isTimeString {
				checkExpr = fmt.Sprintf("%s %s %q", eff, cmp, t.Value)
				kind = "value"
			} else {
				checkExpr = fmt.Sprintf("%s %s %s", eff, cmp, t.Value)
			}
		}

	case f.Type == "string" || f.Type == "*string":
		if t.Name == "eq" || t.Name == "equal" || t.Name == "ne" || t.Name == "neq" {
			checkExpr = fmt.Sprintf("%s %s %q", eff, cmp, t.Value)
			kind = "value"
		} else {
			checkExpr = fmt.Sprintf("utf8.RuneCountInString(%s) %s %s", eff, cmp, t.Value)
			kind = "str"
		}

	case f.IsSlice:
		checkExpr = fmt.Sprintf("len(%s) %s %s", eff, cmp, t.Value)
		kind = "slice"

	case f.Type == "bool" || f.Type == "*bool":
		if (t.Value != "true" && t.Value != "false") || (cmp != "==" && cmp != "!=") {
			return false, fmt.Errorf("wrong comparison for field %s", f.Name)
		}
		expr := eff
		if (cmp == "==" && t.Value == "false") || (cmp == "!=" && t.Value == "true") {
			expr = "!" + expr
		}
		checkExpr = expr
		kind = "value"

	default:
		checkExpr = fmt.Sprintf("%s %s %s", eff, cmp, t.Value)
		kind = "value"
	}

	emitValidationError(buf, checkExpr, fieldSegmentCode(f, ctx.ExtractTags), t.Name, t.Value, kind, "")
	return true, nil
}

// emitValidationError writes a generic ValidationError append. errCapture
// is the name of an already-bound error variable to set as Err, or "" for
// validators that don't capture.
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
	fmt.Fprintf(buf, "\t\t\tKind:  %q,\n", kind)
	if errCapture != "" {
		fmt.Fprintf(buf, "\t\t\tErr:   %s,\n", errCapture)
	}
	buf.WriteString("\t\t})\n\t}\n")
}

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
		buf.WriteString("\t\t\t\tKind:  \"slice\",\n")
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
	buf.WriteString("\t\t\t\tKind: \"slice\",\n")
	buf.WriteString("\t\t\t})\n\t\t}\n")
	fmt.Fprintf(buf, "\t\tseen%s[v] = true\n", f.Name)
	buf.WriteString("\t}\n")
	return true, nil
}

// ---- custom (built-in + user-defined Is<Tag>) ----

type customHandler struct {
	resolve func(tag string) tagResolution
}

func (h customHandler) handle(buf *bytes.Buffer, f field, t Tag, ctx context) (bool, error) {
	res := h.resolve(t.Name)
	if !res.Supported {
		return false, nil
	}
	// `datetime` on time.Time is implicit — it's always a valid time value.
	if t.Name == "datetime" && f.Type != "string" && f.Type != "*string" {
		if f.Type == "time.Time" || (f.IsPointer && f.Type == "*time.Time") {
			fmt.Fprintf(buf, "\t// %s is time.Time, datetime validation is implicit\n", f.effectiveName(ctx.Prefix))
		}
		return true, nil
	}

	pascalTag := shared.PascalCase(t.Name)
	eff := f.effectiveName(ctx.Prefix)

	args := ""
	if t.HasValue {
		for _, part := range strings.Fields(t.Value) {
			args += fmt.Sprintf(", %q", part)
		}
	}

	var checkExpr, errCap string
	if res.ReturnsError {
		checkExpr = fmt.Sprintf("ok, errCapture := %s.Is%s(%s%s); !ok", res.Prefix, pascalTag, eff, args)
		errCap = "errCapture"
	} else {
		checkExpr = fmt.Sprintf("!%s.Is%s(%s%s)", res.Prefix, pascalTag, eff, args)
	}

	kind := requiredKind(f) // same kind buckets as required
	if kind == "value" && (f.Type == "string" || f.Type == "*string") {
		kind = "str"
	}

	emitValidationError(buf, checkExpr, fieldSegmentCode(f, ctx.ExtractTags), t.Name, t.Value, kind, errCap)
	return true, nil
}

// ---- dive (slice / map / struct) ----
//
// Dive handlers all need to recurse into a sub-generator that can produce
// per-element validations. To avoid a circular handler-set construction,
// we hold a deferred reference to the per-field generator.

type diveHandler struct {
	generate func(buf *bytes.Buffer, f field, ctx context) error
}

func (h diveHandler) handle(buf *bytes.Buffer, f field, _ Tag, ctx context) (bool, error) {
	switch {
	case f.IsSlice:
		return h.slice(buf, f, ctx)
	case f.IsMap:
		return h.diveMap(buf, f, ctx)
	default:
		return h.diveStruct(buf, f, ctx)
	}
}

// elementTagsAfter returns the validate tags applied to elements (after
// the dive marker). The dive marker itself is excluded.
func elementTagsAfter(f field) []Tag {
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

func (h diveHandler) slice(buf *bytes.Buffer, f field, ctx context) (bool, error) {
	elementTags := elementTagsAfter(f)
	access := ctx.Prefix + f.Name
	seg := fieldSegmentCode(f, ctx.ExtractTags)

	// Slice of slices: synthesize an inner slice field and recurse so the
	// caller's next `,dive` is re-routed here.
	if strings.HasPrefix(f.ElemType, "[]") && len(elementTags) > 0 {
		inner := strings.TrimPrefix(f.ElemType, "[]")
		var subBuf bytes.Buffer
		synthetic := field{
			FieldInfo: shared.FieldInfo{
				Type:     f.ElemType,
				IsSlice:  true,
				ElemType: inner,
			},
			ParsedTags: elementTags,
		}
		if err := h.generate(&subBuf, synthetic, context{
			StoragePath: ctx.StoragePath,
			Prefix:      "v",
			CaptureErr:  true,
			ExtractTags: ctx.ExtractTags,
			CurrentPkg:  ctx.CurrentPkg,
		}); err != nil {
			return false, err
		}
		if subBuf.Len() == 0 {
			return false, nil
		}
		emitDiveSlicePrimitive(buf, access, seg, subBuf.String())
		return true, nil
	}

	// Slice of primitives with element-level tags.
	if isBasicType(f.ElemType) && len(elementTags) > 0 {
		var subBuf bytes.Buffer
		synthetic := field{
			FieldInfo:  shared.FieldInfo{Type: f.ElemType},
			ParsedTags: elementTags,
		}
		if err := h.generate(&subBuf, synthetic, context{
			StoragePath: ctx.StoragePath,
			Prefix:      "v",
			CaptureErr:  true,
			ExtractTags: ctx.ExtractTags,
			CurrentPkg:  ctx.CurrentPkg,
		}); err != nil {
			return false, err
		}
		if subBuf.Len() == 0 {
			return false, nil
		}
		emitDiveSlicePrimitive(buf, access, seg, subBuf.String())
		return true, nil
	}

	// Slice of anonymous structs — walk inline struct fields.
	if f.AnonymousElem != nil {
		var subBuf bytes.Buffer
		for _, sub := range underlyingStructFields(f.AnonymousElem, ctx.CurrentPkg) {
			if err := h.generate(&subBuf, sub, context{
				StoragePath: ctx.StoragePath,
				Prefix:      "v.",
				CaptureErr:  true,
				ExtractTags: ctx.ExtractTags,
				CurrentPkg:  ctx.CurrentPkg,
			}); err != nil {
				return false, err
			}
		}
		if subBuf.Len() == 0 {
			return false, nil
		}
		emitDiveSlicePrimitive(buf, access, seg, subBuf.String())
		return true, nil
	}

	// Slice of named structs.
	elem := diveElem(f)
	switch {
	case isInterfaceType(elem):
		emitDiveSliceAssertion(buf, access, seg)
		return true, nil
	case willHaveValidate(elem):
		emitDiveSliceDirect(buf, access, seg)
		return true, nil
	default:
		return false, nil
	}
}

func (h diveHandler) diveMap(buf *bytes.Buffer, f field, ctx context) (bool, error) {
	elementTags := elementTagsAfter(f)
	access := ctx.Prefix + f.Name
	seg := fieldSegmentCode(f, ctx.ExtractTags)

	if strings.HasPrefix(f.ElemType, "[]") && len(elementTags) > 0 {
		inner := strings.TrimPrefix(f.ElemType, "[]")
		var subBuf bytes.Buffer
		synthetic := field{
			FieldInfo: shared.FieldInfo{
				Type:     f.ElemType,
				IsSlice:  true,
				ElemType: inner,
			},
			ParsedTags: elementTags,
		}
		if err := h.generate(&subBuf, synthetic, context{
			StoragePath: ctx.StoragePath,
			Prefix:      "v",
			CaptureErr:  true,
			ExtractTags: ctx.ExtractTags,
			CurrentPkg:  ctx.CurrentPkg,
		}); err != nil {
			return false, err
		}
		if subBuf.Len() == 0 {
			return false, nil
		}
		emitDiveMapPrimitive(buf, access, seg, subBuf.String())
		return true, nil
	}

	if isBasicType(f.ElemType) && len(elementTags) > 0 {
		var subBuf bytes.Buffer
		synthetic := field{
			FieldInfo:  shared.FieldInfo{Type: f.ElemType, IsPointer: f.IsPointer},
			ParsedTags: elementTags,
		}
		if err := h.generate(&subBuf, synthetic, context{
			StoragePath: ctx.StoragePath,
			Prefix:      "v",
			CaptureErr:  true,
			ExtractTags: ctx.ExtractTags,
			CurrentPkg:  ctx.CurrentPkg,
		}); err != nil {
			return false, err
		}
		if subBuf.Len() == 0 {
			return false, nil
		}
		emitDiveMapPrimitive(buf, access, seg, subBuf.String())
		return true, nil
	}

	if f.AnonymousElem != nil {
		var subBuf bytes.Buffer
		for _, sub := range underlyingStructFields(f.AnonymousElem, ctx.CurrentPkg) {
			if err := h.generate(&subBuf, sub, context{
				StoragePath: ctx.StoragePath,
				Prefix:      "v.",
				CaptureErr:  true,
				ExtractTags: ctx.ExtractTags,
				CurrentPkg:  ctx.CurrentPkg,
			}); err != nil {
				return false, err
			}
		}
		if subBuf.Len() == 0 {
			return false, nil
		}
		emitDiveMapPrimitive(buf, access, seg, subBuf.String())
		return true, nil
	}

	// Map of named structs.
	elem := diveElem(f)
	switch {
	case isInterfaceType(elem):
		emitDiveMapAssertion(buf, access, seg)
		return true, nil
	case willHaveValidate(elem):
		emitDiveMapDirect(buf, access, seg)
		return true, nil
	default:
		return false, nil
	}
}

func (h diveHandler) diveStruct(buf *bytes.Buffer, f field, ctx context) (bool, error) {
	fieldName := ctx.Prefix + f.Name
	if f.AnonymousType != nil {
		var subBuf bytes.Buffer
		for _, sub := range underlyingStructFields(f.AnonymousType, ctx.CurrentPkg) {
			if err := h.generate(&subBuf, sub, context{
				StoragePath: ctx.StoragePath,
				Prefix:      fieldName + ".",
				CaptureErr:  true,
				ExtractTags: ctx.ExtractTags,
				CurrentPkg:  ctx.CurrentPkg,
			}); err != nil {
				return false, err
			}
		}
		if subBuf.Len() == 0 {
			return false, nil
		}
		buf.WriteString("\t{\n\t\t_start := len(errs)\n")
		buf.WriteString(subBuf.String())
		buf.WriteString("\t\tif _start < len(errs) {\n")
		fmt.Fprintf(buf, "\t\t\tvalidate.PrependFrom(errs, _start, %s)\n", fieldSegmentCode(f, ctx.ExtractTags))
		buf.WriteString("\t\t}\n\t}\n")
		return true, nil
	}

	seg := fieldSegmentCode(f, ctx.ExtractTags)
	elem := diveElem(f)
	switch {
	case isInterfaceType(elem):
		// Interface field: the static type has no Validate, so assert on the
		// dynamic value at runtime.
		if f.IsPointer {
			fmt.Fprintf(buf, "\tif %s != nil {\n", fieldName)
			fmt.Fprintf(buf, "\t\tif vv, ok := any(%s).(interface{ Validate() error }); ok {\n", fieldName)
			buf.WriteString("\t\t\tif subErr := vv.Validate(); subErr != nil {\n")
			fmt.Fprintf(buf, "\t\t\t\tvalidate.AppendSubErrors(&errs, subErr, %s)\n", seg)
			buf.WriteString("\t\t\t}\n\t\t}\n\t}\n")
			return true, nil
		}
		fmt.Fprintf(buf, "\tif vv, ok := any(%s).(interface{ Validate() error }); ok {\n", fieldName)
		buf.WriteString("\t\tif subErr := vv.Validate(); subErr != nil {\n")
		fmt.Fprintf(buf, "\t\t\tvalidate.AppendSubErrors(&errs, subErr, %s)\n", seg)
		buf.WriteString("\t\t}\n\t}\n")
		return true, nil
	case willHaveValidate(elem):
		// Known to have a Validate — call it directly (no runtime assertion).
		if f.IsPointer {
			fmt.Fprintf(buf, "\tif %s != nil {\n", fieldName)
			fmt.Fprintf(buf, "\t\tif subErr := %s.Validate(); subErr != nil {\n", fieldName)
			fmt.Fprintf(buf, "\t\t\tvalidate.AppendSubErrors(&errs, subErr, %s)\n", seg)
			buf.WriteString("\t\t}\n\t}\n")
			return true, nil
		}
		fmt.Fprintf(buf, "\tif subErr := %s.Validate(); subErr != nil {\n", fieldName)
		fmt.Fprintf(buf, "\t\tvalidate.AppendSubErrors(&errs, subErr, %s)\n", seg)
		buf.WriteString("\t}\n")
		return true, nil
	default:
		// Target has no Validate — nothing to dive into.
		return false, nil
	}
}

// emitDiveSlicePrimitive emits an "iterate slice, capture sub-errors,
// prepend an index segment" block. Used for slices of primitives, slices
// of slices, and slices of anon structs (the sub-validations come from
// the caller's pre-built string).
func emitDiveSlicePrimitive(buf *bytes.Buffer, access, seg, subValidations string) {
	if seg != "" {
		// Bind the segment to a local so we can reuse it for every iteration
		// (PrependFrom takes variadic segments).
		fmt.Fprintf(buf, "\t_seg := %s\n", seg)
	}
	fmt.Fprintf(buf, "\tfor i, v := range %s {\n", access)
	buf.WriteString("\t\t_start := len(errs)\n")
	buf.WriteString(subValidations)
	buf.WriteString("\t\tif _start < len(errs) {\n")
	if seg != "" {
		buf.WriteString("\t\t\tvalidate.PrependFrom(errs, _start, _seg, validate.NewIndexSegment(i))\n")
	} else {
		buf.WriteString("\t\t\tvalidate.PrependFrom(errs, _start, validate.NewIndexSegment(i))\n")
	}
	buf.WriteString("\t\t}\n\t}\n")
}

func emitDiveSliceDirect(buf *bytes.Buffer, access, seg string) {
	if seg != "" {
		fmt.Fprintf(buf, "\t_seg := %s\n", seg)
	}
	fmt.Fprintf(buf, "\tfor i, v := range %s {\n", access)
	buf.WriteString("\t\tif subErr := v.Validate(); subErr != nil {\n")
	if seg != "" {
		buf.WriteString("\t\t\tvalidate.AppendSubErrors(&errs, subErr, _seg, validate.NewIndexSegment(i))\n")
	} else {
		buf.WriteString("\t\t\tvalidate.AppendSubErrors(&errs, subErr, validate.NewIndexSegment(i))\n")
	}
	buf.WriteString("\t\t}\n\t}\n")
}

func emitDiveSliceAssertion(buf *bytes.Buffer, access, seg string) {
	if seg != "" {
		fmt.Fprintf(buf, "\t_seg := %s\n", seg)
	}
	fmt.Fprintf(buf, "\tfor i, v := range %s {\n", access)
	buf.WriteString("\t\tif vv, ok := any(v).(interface{ Validate() error }); ok {\n")
	buf.WriteString("\t\t\tif subErr := vv.Validate(); subErr != nil {\n")
	if seg != "" {
		buf.WriteString("\t\t\t\tvalidate.AppendSubErrors(&errs, subErr, _seg, validate.NewIndexSegment(i))\n")
	} else {
		buf.WriteString("\t\t\t\tvalidate.AppendSubErrors(&errs, subErr, validate.NewIndexSegment(i))\n")
	}
	buf.WriteString("\t\t\t}\n\t\t}\n\t}\n")
}

func emitDiveMapPrimitive(buf *bytes.Buffer, access, seg, subValidations string) {
	if seg != "" {
		fmt.Fprintf(buf, "\t_seg := %s\n", seg)
	}
	fmt.Fprintf(buf, "\tfor k, v := range %s {\n", access)
	buf.WriteString("\t\t_start := len(errs)\n")
	buf.WriteString(subValidations)
	buf.WriteString("\t\tif _start < len(errs) {\n")
	if seg != "" {
		buf.WriteString("\t\t\tvalidate.PrependFrom(errs, _start, _seg, validate.NewKeySegment(k))\n")
	} else {
		buf.WriteString("\t\t\tvalidate.PrependFrom(errs, _start, validate.NewKeySegment(k))\n")
	}
	buf.WriteString("\t\t}\n\t}\n")
}

func emitDiveMapDirect(buf *bytes.Buffer, access, seg string) {
	if seg != "" {
		fmt.Fprintf(buf, "\t_seg := %s\n", seg)
	}
	fmt.Fprintf(buf, "\tfor _, v := range %s {\n", access)
	buf.WriteString("\t\tif subErr := v.Validate(); subErr != nil {\n")
	if seg != "" {
		buf.WriteString("\t\t\tvalidate.AppendSubErrors(&errs, subErr, _seg)\n")
	} else {
		buf.WriteString("\t\t\tvalidate.AppendSubErrors(&errs, subErr)\n")
	}
	buf.WriteString("\t\t}\n\t}\n")
}

func emitDiveMapAssertion(buf *bytes.Buffer, access, seg string) {
	if seg != "" {
		fmt.Fprintf(buf, "\t_seg := %s\n", seg)
	}
	fmt.Fprintf(buf, "\tfor _, v := range %s {\n", access)
	buf.WriteString("\t\tif vv, ok := any(v).(interface{ Validate() error }); ok {\n")
	buf.WriteString("\t\t\tif subErr := vv.Validate(); subErr != nil {\n")
	if seg != "" {
		buf.WriteString("\t\t\t\tvalidate.AppendSubErrors(&errs, subErr, _seg)\n")
	} else {
		buf.WriteString("\t\t\t\tvalidate.AppendSubErrors(&errs, subErr)\n")
	}
	buf.WriteString("\t\t\t}\n\t\t}\n\t}\n")
}
