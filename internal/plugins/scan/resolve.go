package scan

import (
	"fmt"

	"gitlab.com/gorib/whisk/internal/generator"
	"gitlab.com/gorib/whisk/internal/shared"
)

// Distinct from the runtime "scan" import, which an extractor in pkg/scan
// would otherwise collide with.
const bodyFmtAlias = "bodyfmt"

type Extractor struct {
	Dir string
	// Dir as configured: a diagnostic names what the author typed, not the abs path
	Label  string
	Import string
}

// Anything beyond json and xml must be backed by
// func Unmarshal<PascalName>([]byte, any) error somewhere in the extractor dir.
func ResolveFormat(name string, ext Extractor) (Format, error) {
	switch name {
	case "", "json":
		return FormatJSON, nil
	case "xml":
		return FormatXML, nil
	}

	fn := "Unmarshal" + shared.PascalCase(name)
	idx, err := shared.IndexFuncs(ext.Dir)
	if err != nil {
		return Format{}, err
	}
	if idx[fn] == nil {
		return Format{}, fmt.Errorf(
			"unknown format %q: no built-in preset, and no func %s in %s %s; point -scan.extractor or whisk.yaml's scan.extractor at the right directory",
			name, fn, ext.Label, shared.SearchNote(ext.Dir, "Unmarshal", idx))
	}
	return Format{
		Tag:       name,
		Import:    generator.Import{Alias: bodyFmtAlias, Path: ext.Import},
		Unmarshal: bodyFmtAlias + "." + fn,
	}, nil
}
