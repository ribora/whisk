package validate

import (
	"bytes"
	"fmt"
	"strings"

	"gitlab.com/gorib/whisk/internal/generator"
	"gitlab.com/gorib/whisk/internal/shared"
	wvalidate "gitlab.com/gorib/whisk/validate"
)

// Options configures the validate plugin.
type Options struct {
	// StoragePath is the absolute directory scanned for user-defined
	// Is<Tag> validators. A tag named "passcode" maps to passcode.go in
	// this directory holding `func IsPasscode(...) bool` (or the error
	// variant `(bool, error)`).
	StoragePath string
	// UserChecksImport is the import path for user-defined Is<Tag>
	// validators discovered in StoragePath. Required when StoragePath
	// contains any matching file; ignored otherwise. Aliased as `checks`
	// in the generated code.
	UserChecksImport string
	// ExtractTags lists struct-tag keys (e.g. "json", "xml") whose values
	// should appear in path segments alongside the Go field name. "go" is
	// implicit. Empty slice ⇒ only "go" is included.
	ExtractTags []string
}

// userChecksAlias is the import alias used for user-defined validators in
// generated code. Picked to be short and obviously not a built-in.
const userChecksAlias = "checks"

// Plugin generates Validate() error methods. Only structs that have at
// least one validate tag, a validate() hook, or an embed contributing a
// Validate get a method — others are skipped (no empty Validate bloat).
type Plugin struct {
	opts Options
}

func New(opts Options) *Plugin { return &Plugin{opts: opts} }

func (p *Plugin) Name() string { return "validate" }

func (p *Plugin) Generate(structs []shared.StructInfo) (generator.Section, error) {
	if len(structs) == 0 {
		return generator.Section{}, nil
	}

	// First pass: promote fields, decide which structs contribute, and
	// gather the tag names referenced so we can resolve handlers.
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

	// Resolve every used tag to a handler-prefix. User-defined Is<Tag>
	// overrides built-ins, so the storage-path check runs first.
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

// builtinFlowTags lists tags whose handlers are wired up unconditionally —
// they don't need to be in the resolutions map (which only carries Is<Tag>
// lookups for custom + built-in validators).
var builtinFlowTags = map[string]struct{}{
	"required": {},
	"unique":   {},
	"dive":     {},
	"min":      {}, "gte": {}, "max": {}, "lte": {},
	"gt": {}, "lt": {}, "eq": {}, "equal": {}, "ne": {}, "neq": {}, "len": {},
}

// structContributes reports whether s gets a generated Validate. A struct
// contributes iff it (or something nested under it) actually carries
// validation — no empty Validate bloat for tagless wrapper structs.
func structContributes(s shared.StructInfo) bool {
	return willHaveValidate(s.Named)
}

// neededImports inspects the generated code to decide which imports the
// final file actually references. errors is always implied (every emitted
// Validate ends in errors.Join), but validate is gated on use: a struct can
// contribute (carry a validate tag) yet emit no validate.* code — e.g.
// required on a non-pointer struct value, which has no zero check.
func neededImports(generatedCode string, hasUserChecks bool, userChecksImport string) []string {
	out := []string{"errors"}
	if strings.Contains(generatedCode, "validate.") {
		out = append(out, "gitlab.com/gorib/whisk/validate")
	}
	if strings.Contains(generatedCode, "fmt.Sprintf") {
		out = append(out, "fmt")
	}
	if strings.Contains(generatedCode, "utf8.RuneCountInString") {
		out = append(out, "unicode/utf8")
	}
	if hasUserChecks && userChecksImport != "" {
		// Aliased imports use a special marker the orchestrator handles;
		// for now embed the alias directly in the import path token. We'll
		// extend Section if/when more plugins need aliasing.
		out = append(out, userChecksAlias+" "+userChecksImport)
	}
	return out
}
