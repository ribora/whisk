package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"gitlab.com/gorib/whisk/internal/plugins/scan"
	"gitlab.com/gorib/whisk/internal/plugins/validate"
)

const configFile = "whisk.yaml"

// A section is the plugin's own Config, so a setting has one owner rather than a
// copy here to drift from.
type projectConfig struct {
	Format   string          `yaml:"format"`
	Scan     scan.Config     `yaml:"scan"`
	Validate validate.Config `yaml:"validate"`
}

// No file is the zero value, not an error: a project that needs nothing writes
// nothing. KnownFields, because a mistyped key can never be intended.
func loadProjectConfig(root string) (projectConfig, error) {
	var cfg projectConfig

	path := filepath.Join(root, configFile)
	fh, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("open %s: %w", configFile, err)
	}
	//nolint:errcheck
	defer fh.Close()

	dec := yaml.NewDecoder(fh)
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		// A comments-only file holds no documents; that is not a broken one.
		if errors.Is(err, io.EOF) {
			return projectConfig{}, nil
		}
		return projectConfig{}, fmt.Errorf("%s: %w", configFile, asSections(err))
	}
	return cfg, nil
}

// yaml names the Go type it could not fill; the reader has sections in front of
// them. An unlisted type keeps yaml's own wording rather than losing the message.
func asSections(err error) error {
	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}
	for i, msg := range typeErr.Errors {
		typeErr.Errors[i] = sectionNames.Replace(msg)
	}
	return typeErr
}

var sectionNames = strings.NewReplacer(
	"in type main.projectConfig", "at the top level",
	"in type scan.Config", `in the "scan" section`,
	"in type validate.Config", `in the "validate" section`,
)

// Flag > whisk.yaml > default. Empty counts as absent, so a file that mentions
// one setting leaves the other defaults alone.
func applyProjectConfig(cfg projectConfig, set map[string]bool, format *string, scanCfg *scan.Config, validateCfg *validate.Config) {
	if cfg.Format != "" && !set["format"] {
		*format = cfg.Format
	}
	if cfg.Scan.ExtractorDir != "" && !set["scan.extractor"] {
		scanCfg.ExtractorDir = cfg.Scan.ExtractorDir
	}
	if cfg.Validate.StorageDir != "" && !set["validate.storage"] {
		validateCfg.StorageDir = cfg.Validate.StorageDir
	}
}

// Visit walks only what was passed on the command line — exactly the set the
// file must not overwrite, with no "was it set" bookkeeping of our own.
func setFlags(fs *flag.FlagSet) map[string]bool {
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	return set
}
