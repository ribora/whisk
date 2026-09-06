package scan

import (
	"maps"
	"slices"
	"strings"

	"gitlab.com/gorib/whisk/internal/shared"
)

// A body format this project can pass to -format.
type FormatName struct {
	Name string
	// Unmarshal<Name> behind it, empty for a built-in preset.
	Func string
	// Set when a function in the extractor directory cannot be reached by any
	// -format value, so Name is empty and only Func is meaningful.
	Unreachable bool
}

// The presets, in the order ResolveFormat answers them.
var builtinFormats = []string{"json", "xml"}

// AvailableFormats is the format surface of one project: the two presets plus
// whatever Unmarshal* the extractor directory holds. Same reason as the tag
// listing — which directory that is depends on a flag, a config file and a
// fallback, so nothing but whisk can answer it.
func AvailableFormats(extractorDir string) ([]FormatName, error) {
	idx, err := shared.IndexFuncs(extractorDir)
	if err != nil {
		return nil, err
	}

	out := make([]FormatName, 0, len(builtinFormats)+len(idx))
	for _, name := range builtinFormats {
		out = append(out, FormatName{Name: name})
	}
	for _, fn := range slices.Sorted(maps.Keys(idx)) {
		name, ok := strings.CutPrefix(fn, "Unmarshal")
		if !ok || name == "" {
			continue
		}
		// Checked by regenerating the name, the way the tag listing does it: a
		// spelling the convention cannot produce is one ResolveFormat would miss.
		lower := strings.ToLower(name)
		if "Unmarshal"+shared.PascalCase(lower) != fn {
			out = append(out, FormatName{Func: fn, Unreachable: true})
			continue
		}
		if slices.Contains(builtinFormats, lower) {
			// A preset wins: ResolveFormat answers json and xml before it looks
			// in the directory, so this function is never called.
			out = append(out, FormatName{Func: fn, Unreachable: true})
			continue
		}
		out = append(out, FormatName{Name: lower, Func: fn})
	}
	return out, nil
}
