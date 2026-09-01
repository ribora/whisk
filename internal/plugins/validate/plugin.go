package validate

import (
	"bytes"
	"fmt"
	"strings"

	"gitlab.com/gorib/whisk/internal/generator"
	"gitlab.com/gorib/whisk/internal/shared"
	wvalidate "gitlab.com/gorib/whisk/validate"
)

type Options struct {
	// validate:"passcode" -> <StoragePath>/passcode.go, func IsPasscode(…) bool
	StoragePath string
	// import path of StoragePath; required once anything is found there
	UserChecksImport string
	// tag keys whose names join "go" in a path segment: json, xml, …
	ExtractTags []string
}

// Short, and obviously not a built-in.
const userChecksAlias = "checks"

// Only a struct that carries validation gets a method: an empty Validate on every
// tagless wrapper is noise the reader has to skip past.
type Plugin struct {
	opts Options
}

func New(opts Options) *Plugin { return &Plugin{opts: opts} }

func (p *Plugin) Name() string { return "validate" }

func (p *Plugin) Generate(structs []shared.StructInfo) (generator.Section, error) {
	if len(structs) == 0 {
		return generator.Section{}, nil
	}

	// Handlers are resolved from the set of tags actually used, so every struct is
	// walked before any of them is rendered.
	type prepared struct {
		Name     string
		PkgPath  string
		HasHook  bool
		Fields   []field
		FieldRaw shared.StructInfo
	}
	var prepList []prepared
	usedTags := map[string]struct{}{}
	for _, s := range structs {
		fields := make([]field, len(s.Fields))
		for i, f := range s.Fields {
			fields[i] = promote(f, s.PkgPath)
		}
		if !structContributes(s) {
			continue
		}
		prepList = append(prepList, prepared{
			Name:     s.Name,
			PkgPath:  s.PkgPath,
			HasHook:  s.HasValidateHook,
			Fields:   fields,
			FieldRaw: s,
		})
		for _, f := range fields {
			for _, t := range f.ParsedTags {
				if t.Name != "" {
					usedTags[t.Name] = struct{}{}
				}
			}
		}
	}
	if len(prepList) == 0 {
		return generator.Section{}, nil
	}

	// The storage path is consulted first: a user's Is<Tag> is meant to win over
	// the built-in of the same name.
	resolutions := map[string]tagResolution{}
	hasUserChecks := false
	for tag := range usedTags {
		if _, isBuiltinFlow := builtinFlowTags[tag]; isBuiltinFlow {
			continue
		}
		if isCustom, retErr := isCustomValidator(tag, p.opts.StoragePath); isCustom {
			resolutions[tag] = tagResolution{Prefix: userChecksAlias, ReturnsError: retErr, Supported: true}
			hasUserChecks = true
			continue
		}
		if retErr, ok := wvalidate.BuiltIns[tag]; ok {
			resolutions[tag] = tagResolution{Prefix: "validate", ReturnsError: retErr, Supported: true}
			continue
		}
	}

	gen := newDispatcher(resolutions)

	var code bytes.Buffer
	for _, ps := range prepList {
		if err := emitValidateMethod(&code, gen, ps.Name, ps.PkgPath, ps.HasHook, ps.Fields, p.opts.ExtractTags); err != nil {
			return generator.Section{}, fmt.Errorf("struct %s: %w", ps.Name, err)
		}
	}

	imports := neededImports(code.String(), hasUserChecks, p.opts.UserChecksImport)
	return generator.Section{Code: code.Bytes(), Imports: imports}, nil
}

// Wired unconditionally, so they never need resolving to an Is<Tag>.
var builtinFlowTags = map[string]struct{}{
	"required": {},
	"unique":   {},
	"dive":     {},
	"min":      {}, "gte": {}, "max": {}, "lte": {},
	"gt": {}, "lt": {}, "eq": {}, "equal": {}, "ne": {}, "neq": {}, "len": {},
}

// True when s, or anything nested under it, actually carries validation.
func structContributes(s shared.StructInfo) bool {
	return willHaveValidate(s.Named)
}

// Read off the emitted text rather than predicted: a struct can carry a tag and
// still emit no validate.* call — required on a struct value has no zero check.
func neededImports(generatedCode string, hasUserChecks bool, userChecksImport string) []generator.Import {
	out := []generator.Import{{Path: "errors"}}
	if strings.Contains(generatedCode, "validate.") {
		out = append(out, generator.Import{Path: "gitlab.com/gorib/whisk/validate"})
	}
	if strings.Contains(generatedCode, "fmt.Sprintf") {
		out = append(out, generator.Import{Path: "fmt"})
	}
	if strings.Contains(generatedCode, "utf8.RuneCountInString") {
		out = append(out, generator.Import{Path: "unicode/utf8"})
	}
	if hasUserChecks && userChecksImport != "" {
		out = append(out, generator.Import{Alias: userChecksAlias, Path: userChecksImport})
	}
	return out
}
