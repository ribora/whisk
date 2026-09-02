package validate

import "flag"

// Resolved into Options by the composition root, together with the inputs no
// single plugin owns. The yaml key is whisk.yaml's spelling of the same flag, so
// both live with the plugin that reads them.
type Config struct {
	StorageDir string `yaml:"storage"`
}

func DefaultConfig() Config {
	return Config{StorageDir: "pkg/validate"}
}

func (c *Config) BindFlags(fs *flag.FlagSet, prefix string) {
	fs.StringVar(&c.StorageDir, prefix+".storage", c.StorageDir,
		"directory of user-defined Is<Tag> validators (relative to project root)")
}
