package validate

import (
	"fmt"
	"go/types"
	"slices"
	"strings"

	"github.com/ribora/whisk/internal/shared"
)

type field struct {
	shared.FieldInfo

	// Carries a dive that promote added when nobody wrote one.
	ParsedTags []Tag
	// True when that dive is the added one.
	IsStruct bool
}

func promote(f shared.FieldInfo, pkgPath string) field {
	tags := parseTags(f.Tag.Get("validate"))

	// A field whose rules live in its own Validate is dived into without being
	// asked. Requiring an explicit ,dive would make forgetting it the default, and
	// a forgotten dive is silence rather than an error.
	added := !hasTag(tags, "dive") && nested(f, pkgPath)
	if added {
		tags = append(tags, Tag{Name: "dive"})
	}

	return field{FieldInfo: f, ParsedTags: tags, IsStruct: added}
}

func hasTag(tags []Tag, name string) bool {
	return slices.ContainsFunc(tags, func(t Tag) bool { return t.Name == name })
}

// time.Time is the exception: a struct, but one the rules here compare directly.
func nested(f shared.FieldInfo, pkgPath string) bool {
	base := unwrapToBaseTypeText(f.TypeRef, pkgPath)
	return base != "" && !isBasicType(base) && f.Type != "time.Time"
}

// A value the added dive walks into rather than tests: a struct, a collection of
// them, a pointer to one. A named scalar is not one of those — type ID string has
// no fields — so it keeps the guard its underlying type calls for, which is what
// nested cannot tell from the type's spelling alone.
func (f field) divesIntoValue() bool {
	return f.IsStruct && basicUnder(f.TypeRef) == ""
}

// The built-ins and utf8.RuneCountInString take a basic type exactly, so a named
// one is converted on the way in. A plain string is left alone rather than
// dressed in a no-op conversion nobody wants to read.
func (f field) asBasic(expr string) string {
	under := basicUnder(f.TypeRef)
	if under == "" || strings.TrimPrefix(f.Type, "*") == under {
		return expr
	}
	return under + "(" + expr + ")"
}

// A pointer is dereferenced, so the caller compares values either way.
func (f field) effectiveName(prefix string) string {
	name := prefix + f.Name
	if f.IsPointer {
		return "*" + name
	}
	return name
}

// How a field is compared against its zero value, read off what the type is
// underneath rather than what it is called: type ID string is a string here, and
// treating it as a struct is how it lost both its guard and its required check.
type shape int

const (
	shapeNone shape = iota // a struct or an interface: no zero worth testing
	shapeLen               // slice, map
	shapeTime
	shapeString
	shapeBool
	shapeNumber
)

// A pointer is looked through: a check dereferences it before comparing, so *ID
// compares as the string it points at. Whether the pointer itself is nil is a
// separate question, asked by the two callers that care.
func (f field) shape() shape {
	switch {
	case f.IsSlice || f.IsMap:
		return shapeLen
	case f.Type == "time.Time" || f.Type == "*time.Time":
		return shapeTime
	}
	switch basicUnder(f.TypeRef) {
	case "":
		return shapeNone
	case "string":
		return shapeString
	case "bool":
		return shapeBool
	}
	return shapeNumber
}

// zeroCondition returns the is-zero test for fieldName. ok=false means the
// type has no meaningful zero check; bool is a hard error (use equal, not
// required).
func (f field) zeroCondition(fieldName string) (string, bool, error) {
	if f.IsSlice || f.IsMap {
		return fmt.Sprintf("len(%s) == 0", fieldName), true, nil
	}
	// A pointer's own zero is nil, whatever it points at.
	if f.IsPointer {
		return fieldName + " == nil", true, nil
	}
	switch f.shape() {
	case shapeTime:
		return fieldName + ".IsZero()", true, nil
	case shapeString:
		return fieldName + ` == ""`, true, nil
	case shapeNumber:
		return fieldName + " == 0", true, nil
	case shapeBool:
		return "", false, fmt.Errorf("required tag doesn't make sense for bool field %q, use equal instead", f.Name)
	}
	return "", false, nil
}

// The guard every tag other than required sits inside, so an empty optional field
// stays quiet. bool gets none: false is a value, not an absence.
func (f field) presenceGuard(fieldName string) (string, bool) {
	if f.IsSlice || f.IsMap {
		return fmt.Sprintf("len(%s) > 0", fieldName), true
	}
	if f.IsPointer {
		return fieldName + " != nil", true
	}
	switch f.shape() {
	case shapeTime:
		return "!" + fieldName + ".IsZero()", true
	case shapeString:
		return fieldName + ` != ""`, true
	case shapeNumber:
		return fieldName + " != 0", true
	}
	return "", false
}

func underlyingStructFields(st *types.Struct, pkgPath string) []field {
	infos := shared.Fields(st, pkgPath)
	out := make([]field, len(infos))
	for i, fi := range infos {
		out[i] = promote(fi, pkgPath)
	}
	return out
}

// The leading "*" is ignored: a *string is still a string for this question.
func isBasicType(t string) bool {
	t = strings.TrimPrefix(t, "*")
	switch t {
	case "string", "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "bool", "byte", "rune":
		return true
	}
	return false
}

// The basic type a named one is declared over, through one pointer: both ID and
// *ID answer "string" for type ID string. Empty for everything else.
func basicUnder(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if t == nil {
		return ""
	}
	b, ok := t.Underlying().(*types.Basic)
	if !ok {
		return ""
	}
	return b.Name()
}

func unwrapToBaseTypeText(t types.Type, pkgPath string) string {
	for {
		switch x := t.(type) {
		case *types.Pointer:
			t = x.Elem()
		case *types.Slice:
			t = x.Elem()
		case *types.Array:
			t = x.Elem()
		case *types.Map:
			t = x.Elem()
		default:
			return shared.TypeText(t, pkgPath)
		}
	}
}
