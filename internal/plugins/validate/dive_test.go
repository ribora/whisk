package validate_test

import (
	"strings"
	"testing"

	"github.com/ribora/whisk/internal/plugins/validate"
)

// Diving is only useful if the error says which element failed. The locator used to
// be spelled out per container-and-element combination, and the two map-of-struct
// ones were written without it, so a failing entry reported its map and nothing else.
func TestDiveKeepsTheElementLocator(t *testing.T) {
	sec := runValidate(t, `package sample

type Item struct {
	Name string `+"`validate:\"required\"`"+`
}

type Named interface{ Validate() error }

type Order struct {
	MapOfStructs    map[string]Item   `+"`validate:\"dive\"`"+`
	MapOfPrimitives map[string]string `+"`validate:\"dive,min=2\"`"+`
	MapOfIfaces     map[string]Named  `+"`validate:\"dive\"`"+`
	SliceOfStructs  []Item            `+"`validate:\"dive\"`"+`
	SliceOfPrims    []string          `+"`validate:\"dive,min=2\"`"+`
	SliceOfIfaces   []Named           `+"`validate:\"dive\"`"+`
}
`, validate.Options{})

	code := string(sec.Code)

	if n := strings.Count(code, "validate.NewKeySegment(k)"); n != 3 {
		t.Errorf("%d map dives carry the key, want 3:\n%s", n, code)
	}
	if n := strings.Count(code, "validate.NewIndexSegment(i)"); n != 3 {
		t.Errorf("%d slice dives carry the index, want 3:\n%s", n, code)
	}

	// A loop that discards its key cannot report one, whatever the segments say.
	if strings.Contains(code, "for _, v := range s.Map") {
		t.Errorf("a map dive discards its key:\n%s", code)
	}
}
