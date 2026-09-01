package scan

import "flag"

type Config struct {
	// -format=toml -> <ExtractorDir>/toml.go, func UnmarshalToml([]byte, any) error
	ExtractorDir string
}

func DefaultConfig() Config {
	return Config{ExtractorDir: "pkg/scan"}
}

func (c *Config) BindFlags(fs *flag.FlagSet, prefix string) {
	fs.StringVar(&c.ExtractorDir, prefix+".extractor", c.ExtractorDir,
		"directory of custom body-format extractors (Unmarshal<Format>), relative to project root")
}
