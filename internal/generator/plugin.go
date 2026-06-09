package generator

import (
	"gitlab.com/gorib/whisk/internal/shared"
)

// Plugin is a code-generation unit: it inspects the parsed structs and emits
// one section of the final file. The orchestrator concatenates sections,
// dedupes imports, and formats once. Plugins MUST be deterministic.
type Plugin interface {
	Name() string
	// Generate returns a zero-value Section when the plugin has nothing to
	// add for these structs.
	Generate(structs []shared.StructInfo) (Section, error)
}

// Section is one plugin's contribution. Imports are full paths; the
// orchestrator dedupes and sorts them across plugins. Warnings are
// non-fatal — surfaced, but generation continues.
type Section struct {
	Code     []byte
	Imports  []string
	Warnings []string
}
