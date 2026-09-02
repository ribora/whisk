package validate

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"gitlab.com/gorib/whisk/internal/generator"
	"gitlab.com/gorib/whisk/internal/shared"
	wvalidate "gitlab.com/gorib/whisk/validate"
)

type Options struct {
	// validate:"passcode" -> func IsPasscode(…) bool in any file under StoragePath
	StoragePath string
	// StoragePath as configured: a diagnostic names what the author typed
	StorageLabel string
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
	// So a failed resolve names a field instead of leaving the author to grep.
	firstUse := map[string]string{}
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
				if t.Name == "" {
					continue
				}
				usedTags[t.Name] = struct{}{}
				if _, seen := firstUse[t.Name]; !seen {
					firstUse[t.Name] = s.Name + "." + f.Name
				}
			}
		}
	}
	if len(prepList) == 0 {
		return generator.Section{}, nil
	}

	idx, err := shared.IndexFuncs(p.opts.StoragePath)
	if err != nil {
		return generator.Section{}, err
	}

	// The storage path is consulted first: a user's Is<Tag> is meant to win over
	// the built-in of the same name. Sorted, or warnings and errors would come out
	// in a different order on every run.
	resolutions := map[string]tagResolution{}
	hasUserChecks := false
	var warnings []string
	var unknown []error
	for _, tag := range slices.Sorted(maps.Keys(usedTags)) {
		if _, isBuiltinFlow := builtinFlowTags[tag]; isBuiltinFlow {
			continue
		}
		if isCustom, retErr := isCustomValidator(idx, tag); isCustom {
			resolutions[tag] = tagResolution{Prefix: userChecksAlias, ReturnsError: retErr, Supported: true}
			hasUserChecks = true
			// The one case where the model's reader cannot tell which check runs.
			if _, shadowed := wvalidate.BuiltIns[tag]; shadowed {
				warnings = append(warnings, fmt.Sprintf("%s.%s overrides the built-in %q validator",
					userChecksAlias, customFuncName(tag), tag))
			}
			continue
		}
		if retErr, ok := wvalidate.BuiltIns[tag]; ok {
			resolutions[tag] = tagResolution{Prefix: "validate", ReturnsError: retErr, Supported: true}
			continue
		}
		unknown = append(unknown, p.unknownTagError(tag, firstUse[tag], idx))
	}
	if len(unknown) > 0 {
		return generator.Section{}, errors.Join(unknown...)
	}

	gen := newDispatcher(resolutions)

	var code bytes.Buffer
	for _, ps := range prepList {
		if err := emitValidateMethod(&code, gen, ps.Name, ps.PkgPath, ps.HasHook, ps.Fields, p.opts.ExtractTags); err != nil {
			return generator.Section{}, fmt.Errorf("struct %s: %w", ps.Name, err)
		}
	}

	imports := neededImports(code.String(), hasUserChecks, p.opts.UserChecksImport)
	return generator.Section{Code: code.Bytes(), Imports: imports, Warnings: warnings}, nil
}

// Raised here rather than in the dispatcher: this is the only place that knows
// where it looked.
func (p *Plugin) unknownTagError(tag, use string, idx shared.FuncIndex) error {
	where := ""
	if use != "" {
		where = use + ": "
	}
	label := cmp.Or(p.opts.StorageLabel, p.opts.StoragePath)
	if label == "" {
		return fmt.Errorf("%sunknown validator tag %q: not a built-in, and no storage directory is configured to hold %s",
			where, tag, customFuncName(tag))
	}
	return fmt.Errorf("%sunknown validator tag %q: not a built-in, and no func %s in %s %s; point -validate.storage or whisk.yaml's validate.storage at the right directory",
		where, tag, customFuncName(tag), label, shared.SearchNote(p.opts.StoragePath, "Is", idx))
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
