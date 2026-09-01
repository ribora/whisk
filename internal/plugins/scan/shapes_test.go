package scan_test

import (
	"strings"
	"testing"

	"gitlab.com/gorib/whisk/internal/plugins/scan"
)

// A shape the emitter cannot build from text has one correct outcome: the TODO
// marker naming the type, so the author finishes it in a parse() hook. Maps and
// arrays used to take the slice path instead — shared.FieldInfo reports an ElemType
// for both — and `make([]T, …)` was written into a field that is neither.
func TestUnsupportedShapesGetAMarker(t *testing.T) {
	sec := runScan(t, `package sample

type Req struct {
	Map   map[string]string `+"`scan:\"query=map\"`"+`
	Fixed [3]string         `+"`scan:\"query=fixed\"`"+`
	PtrSl *[]string         `+"`scan:\"query=ptrsl\"`"+`
	Slice []string          `+"`scan:\"query=slice\"`"+`
}
`, scan.FormatJSON)

	code := string(sec.Code)

	for _, want := range []string{
		"cannot convert string to map[string]string for Map",
		"cannot convert string to [3]string for Fixed",
		"cannot convert string to *[]string for PtrSl",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("missing marker %q in\n%s", want, code)
		}
	}
	// The bug was never a missing marker but the code emitted next to it.
	for _, never := range []string{"s.Map = make(", "s.Fixed = make("} {
		if strings.Contains(code, never) {
			t.Errorf("%q is still built as a slice:\n%s", never, code)
		}
	}
	// The shape that genuinely is a slice still works.
	if !strings.Contains(code, "s.Slice = make([]string,") {
		t.Errorf("a real slice stopped being built:\n%s", code)
	}
}
