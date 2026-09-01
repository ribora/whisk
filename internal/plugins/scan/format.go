package scan

import "gitlab.com/gorib/whisk/internal/generator"

// Only the body unmarshal uses this; a scoped lookup is format-agnostic.
type Format struct {
	Tag       string // struct-tag key for the format; also drives validate path-segment names
	Import    generator.Import
	Unmarshal string // call expression emitted verbatim, e.g. "json.Unmarshal"
}

var (
	FormatJSON = Format{Tag: "json", Import: generator.Import{Path: "encoding/json"}, Unmarshal: "json.Unmarshal"}
	FormatXML  = Format{Tag: "xml", Import: generator.Import{Path: "encoding/xml"}, Unmarshal: "xml.Unmarshal"}
)
