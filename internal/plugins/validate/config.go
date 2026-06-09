package validate

import "flag"

// Config is the validate plugin's CLI surface (the -validate.* flags). The
// composition root resolves it — plus cross-cutting inputs like the body
// format — into Options before constructing the plugin.
type Config struct {
	StorageDir string
}

func DefaultConfig() Config {
	return Config{StorageDir: "pkg/validate"}
}

// BindFlags registers the plugin's flags under prefix (-<prefix>.storage).
func (c *Config) BindFlags(fs *flag.FlagSet, prefix string) {
	fs.StringVar(&c.StorageDir, prefix+".storage", c.StorageDir,
		"directory of user-defined Is<Tag> validators (relative to project root)")
}
