package validate

import (
	"maps"
	"slices"
	"strings"
	"unicode"

	"gitlab.com/gorib/whisk/internal/shared"
	wvalidate "gitlab.com/gorib/whisk/validate"
)

// Origin says which of the three places a tag's check comes from. The order is
// the resolution order the generator uses, so a listing sorted by it reads the
// way a lookup happens.
type Origin int

const (
	// Wired by the emitter itself — no Is<Tag> behind it.
	OriginFlow Origin = iota
	// Is<Tag> in whisk's own validate package.
	OriginBuiltIn
	// Is<Tag> in this project's storage directory, which wins over a built-in
	// of the same name.
	OriginCustom
)

// SupportedTag is one tag this project can put in a `validate` struct tag.
type SupportedTag struct {
	Name   string
	Origin Origin
	// Is<Tag> behind it, empty for a flow tag.
	Func string
	// A custom tag that replaces the built-in of the same name. The generator
	// warns about this at generation time; a listing that hid it would be
	// claiming the built-in still runs.
	Shadows bool
	// Set when a function in the storage directory cannot be reached by any tag,
	// so Name is empty and only Func is meaningful.
	Unreachable bool
}

// Available is the tag surface of one project: the flow tags, the built-ins, and
// whatever Is* the storage directory holds. Only whisk can compute this — the
// built-ins are its own and the custom ones are the project's, and which
// directory is "the project's" depends on a flag, a config file and a fallback.
//
// An empty or missing directory is not an error: it means the project has added
// nothing, which is the common case.
func Available(storagePath string) ([]SupportedTag, error) {
	idx, err := shared.IndexFuncs(storagePath)
	if err != nil {
		return nil, err
	}

	out := make([]SupportedTag, 0, len(builtinFlowTags)+len(wvalidate.BuiltIns)+len(idx))
	for _, name := range slices.Sorted(maps.Keys(builtinFlowTags)) {
		out = append(out, SupportedTag{Name: name, Origin: OriginFlow})
	}

	custom := customTags(idx)
	for _, name := range slices.Sorted(maps.Keys(wvalidate.BuiltIns)) {
		if slices.ContainsFunc(custom, func(t SupportedTag) bool { return t.Name == name }) {
			// Listed once, under the origin that actually answers it.
			continue
		}
		out = append(out, SupportedTag{Name: name, Origin: OriginBuiltIn, Func: customFuncName(name)})
	}
	return append(out, custom...), nil
}

// The index is keyed by function name, and a tag is what reaches a function —
// so each candidate is turned back into a tag and checked by generating the
// function name again. A name the convention cannot produce is reported as
// unreachable rather than guessed at: the generator would never find it either.
func customTags(idx shared.FuncIndex) []SupportedTag {
	var out []SupportedTag
	for _, fn := range slices.Sorted(maps.Keys(idx)) {
		if !strings.HasPrefix(fn, "Is") || fn == "Is" {
			continue
		}
		tag := tagOf(fn)
		if tag == "" || customFuncName(tag) != fn {
			out = append(out, SupportedTag{Origin: OriginCustom, Func: fn, Unreachable: true})
			continue
		}
		_, shadows := wvalidate.BuiltIns[tag]
		out = append(out, SupportedTag{Name: tag, Origin: OriginCustom, Func: fn, Shadows: shadows})
	}
	return out
}

// The inverse of "Is" + PascalCase: split where a word starts, join with "_".
// IsCompanyId -> company_id, IsPasscode -> passcode.
func tagOf(fn string) string {
	var words []string
	for _, r := range strings.TrimPrefix(fn, "Is") {
		if unicode.IsUpper(r) || len(words) == 0 {
			words = append(words, "")
		}
		words[len(words)-1] += string(unicode.ToLower(r))
	}
	return strings.Join(words, "_")
}
