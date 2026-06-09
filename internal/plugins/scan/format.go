package scan

// Format selects the body decoder. Only the generated body-unmarshal block
// uses it; scoped lookups are format-agnostic.
type Format struct {
	Tag       string // struct-tag key for the format; also drives validate path-segment names
	Import    string
	Unmarshal string // call expression emitted verbatim, e.g. "json.Unmarshal"
}

var (
	FormatJSON = Format{Tag: "json", Import: "encoding/json", Unmarshal: "json.Unmarshal"}
	FormatXML  = Format{Tag: "xml", Import: "encoding/xml", Unmarshal: "xml.Unmarshal"}
)
