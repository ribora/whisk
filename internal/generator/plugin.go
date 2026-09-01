package generator

import (
	"gitlab.com/gorib/whisk/internal/shared"
)

// One section of the final file. Output must be deterministic — it is compared
// against goldens and committed.
type Plugin interface {
	Name() string
	// A zero Section means nothing to add.
	Generate(structs []shared.StructInfo) (Section, error)
}

// A warning does not stop generation.
type Section struct {
	Code     []byte
	Imports  []Import
	Warnings []string
}

// An import as the generated file needs it. The alias used to travel inside the
// path, packed as "alias path" and split apart again on the way out.
type Import struct {
	Alias string
	Path  string
}
