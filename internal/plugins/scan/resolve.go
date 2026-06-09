package scan

import (
	"fmt"

	"gitlab.com/gorib/whisk/internal/shared"
)

// bodyFmtAlias is the import alias for a custom extractor package in generated
// code — kept distinct from the runtime "scan" import (which a pkg/scan
// extractor would otherwise collide with).
const bodyFmtAlias = "bodyfmt"

// ResolveFormat returns the body Format for a -format name. json and xml are
// built-in presets; any other name must be backed by a custom extractor — a
//
//	func Unmarshal<PascalName>(b []byte, v any) error
//
// declared in extractorDir/<name>.go. extractorImport is that directory's
// import path, emitted (aliased) in the generated file.
func ResolveFormat(name, extractorDir, extractorImport string) (Format, error) {
	switch name {
	case "", "json":
		return FormatJSON, nil
	case "xml":
		return FormatXML, nil
	}
	fn := "Unmarshal" + shared.PascalCase(name)
	if shared.FindFunc(extractorDir, name+".go", fn) == nil {
		return Format{}, fmt.Errorf("unknown format %q: no builtin preset, and no func %s in %s/%s.go", name, fn, extractorDir, name)
	}
	return Format{
		Tag:       name,
		Import:    bodyFmtAlias + " " + extractorImport,
		Unmarshal: bodyFmtAlias + "." + fn,
	}, nil
}
