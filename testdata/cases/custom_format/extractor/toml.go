package extractor

// UnmarshalToml is a stub custom body-format extractor: a real one would
// delegate to a TOML library. The Unmarshal<Format> name and the
// (b []byte, v any) error signature are the convention whisk looks for.
func UnmarshalToml(b []byte, v any) error { return nil }
