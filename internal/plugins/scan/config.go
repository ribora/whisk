package scan

import "flag"

type Config struct {
	// -format=toml -> func UnmarshalToml([]byte, any) error under ExtractorDir
	ExtractorDir string `yaml:"extractor"`
}

func DefaultConfig() Config {
	return Config{ExtractorDir: "pkg/scan"}
}

func (c *Config) BindFlags(fs *flag.FlagSet, prefix string) {
	fs.StringVar(&c.ExtractorDir, prefix+".extractor", c.ExtractorDir,
		"directory of custom body-format extractors (Unmarshal<Format>), relative to project root")
}
