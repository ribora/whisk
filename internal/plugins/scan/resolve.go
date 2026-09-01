package scan

import (
	"fmt"

	"gitlab.com/gorib/whisk/internal/generator"
	"gitlab.com/gorib/whisk/internal/shared"
)

// Distinct from the runtime "scan" import, which an extractor in pkg/scan
// would otherwise collide with.
const bodyFmtAlias = "bodyfmt"

// Anything beyond json and xml must be backed by
// func Unmarshal<PascalName>([]byte, any) error in extractorDir/<name>.go.
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
		Import:    generator.Import{Alias: bodyFmtAlias, Path: extractorImport},
		Unmarshal: bodyFmtAlias + "." + fn,
	}, nil
}
