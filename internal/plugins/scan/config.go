package scan

import "flag"

// Config is the scan plugin's CLI surface (the -scan.* flags).
type Config struct {
	// ExtractorDir is the directory, relative to the project root, scanned for
	// custom body-format decoders (Unmarshal<Format> functions) when -format
	// names a non-builtin. Bound to -<prefix>.extractor.
	ExtractorDir string
}

// DefaultConfig returns the scan config with built-in defaults. By convention
// custom extractors live in pkg/scan at the module root.
func DefaultConfig() Config {
	return Config{ExtractorDir: "pkg/scan"}
}

// BindFlags registers this plugin's flags on fs under the given prefix,
// following the -<plugin>.<flag> convention (here: -scan.extractor).
func (c *Config) BindFlags(fs *flag.FlagSet, prefix string) {
	fs.StringVar(&c.ExtractorDir, prefix+".extractor", c.ExtractorDir,
		"directory of custom body-format extractors (Unmarshal<Format>), relative to project root")
}
