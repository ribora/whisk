package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Values, not commented hints: the paths being invisible is the complaint this
// file answers, and a comment is a weaker form of visible.
const configTemplate = `# whisk.yaml — project settings for the whisk generator.
# These are whisk's defaults, written out so the paths are visible. Drop a line
# to follow the default instead; a flag on a go:generate line overrides both.

format: json             # json | xml | <custom, see scan.extractor>

scan:
  extractor: pkg/scan    # dir holding func Unmarshal<Format>([]byte, any) error

validate:
  storage: pkg/validate  # dir holding func Is<Tag>(value, args ...string) bool
`

func runInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := findProjectRoot(cwd)
	if err != nil {
		return err
	}

	// Never overwrite — edited settings are not the generator's to discard — but
	// say what was left alone, or a rerun looks like a run that did nothing. An
	// unreadable target is the one case where skipping would be wrong.
	path := filepath.Join(root, configFile)
	switch _, err := os.Stat(path); {
	case err == nil:
		fmt.Fprintf(os.Stderr, "whisk: = %s — exists, left alone\n", path)
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(configTemplate), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "whisk: + %s\n", path)
	return nil
}
