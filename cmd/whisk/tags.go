package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/gorib/whisk/internal/plugins/scan"
	"gitlab.com/gorib/whisk/internal/plugins/validate"
)

// source names where a directory setting came from, because a listing of what a
// project supports is only useful next to the answer "and why that directory".
type source string

const (
	fromFlag     source = "-%s on this command line"
	fromConfig   source = "%s in " + configFile
	fromFallback source = "the fallback for %s"
)

// runTags prints the tag and format surface of the project the working directory
// belongs to. It resolves the two directories the same way generation does, and
// says which of the three sources won: the whole reason this cannot be answered
// by reading the documentation is that the answer is per-project.
//
// One thing it cannot see: -validate.storage passed on an individual
// //go:generate line. Those are per-directive, so the listing says so rather
// than implying the resolved directory holds everywhere.
func listTags(set map[string]bool, scanCfg scan.Config, validateCfg validate.Config) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	root, err := findProjectRoot(cwd)
	if err != nil {
		return err
	}
	cfg, err := loadProjectConfig(root)
	if err != nil {
		return err
	}
	return writeTagReport(os.Stdout, root, cfg, set, scanCfg, validateCfg)
}

func writeTagReport(w io.Writer, root string, cfg projectConfig, set map[string]bool,
	scanCfg scan.Config, validateCfg validate.Config) error {
	storage, storageFrom := resolve("validate.storage", validateCfg.StorageDir, cfg.Validate.StorageDir, set)
	extractor, extractorFrom := resolve("scan.extractor", scanCfg.ExtractorDir, cfg.Scan.ExtractorDir, set)

	tags, err := validate.Available(filepath.Join(root, storage))
	if err != nil {
		return err
	}
	formats, err := scan.AvailableFormats(filepath.Join(root, extractor))
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(w, "# what a `validate` tag may say in %s\n", root)
	writeGroup(w, "wired into the generated code — no function behind them", tags, validate.OriginFlow)
	writeGroup(w, "whisk's own", tags, validate.OriginBuiltIn)
	_, _ = fmt.Fprintf(w, "\n  this project's, from %s (%s)\n", storage, storageFrom)
	if !writeTags(w, tags, validate.OriginCustom) {
		_, _ = fmt.Fprintf(w, "    nothing there yet — add func Is<Tag>(value string, args ...string) bool\n")
	}

	_, _ = fmt.Fprintf(w, "\n# what -format may say\n")
	for _, f := range formats {
		switch {
		case f.Unreachable:
			_, _ = fmt.Fprintf(w, "    %-14s — no -format value reaches this function\n", f.Func)
		case f.Func == "":
			_, _ = fmt.Fprintf(w, "    %-14s   built in\n", f.Name)
		default:
			_, _ = fmt.Fprintf(w, "    %-14s   %s\n", f.Name, f.Func)
		}
	}
	_, _ = fmt.Fprintf(w, "\n  custom formats come from %s (%s)\n", extractor, extractorFrom)

	_, _ = fmt.Fprintf(w, "\nA -validate.storage on an individual //go:generate line overrides this, so a\n")
	_, _ = fmt.Fprintf(w, "package whose directive carries one resolves against a different directory.\n")
	return nil
}

// Same precedence as applyProjectConfig, reported rather than applied: flag,
// then the file, then the default the flag was registered with.
func resolve(key, current, fromFile string, set map[string]bool) (dir string, from string) {
	switch {
	case set[key]:
		return current, fmt.Sprintf(string(fromFlag), key)
	case fromFile != "":
		return fromFile, fmt.Sprintf(string(fromConfig), key)
	default:
		return current, fmt.Sprintf(string(fromFallback), key)
	}
}

func writeGroup(w io.Writer, heading string, tags []validate.SupportedTag, origin validate.Origin) {
	_, _ = fmt.Fprintf(w, "\n  %s\n", heading)
	writeTags(w, tags, origin)
}

// Reports whether it wrote anything, so a caller can say "nothing yet" instead
// of leaving a blank heading that reads like a bug.
func writeTags(w io.Writer, tags []validate.SupportedTag, origin validate.Origin) bool {
	var names []string
	wrote := false
	for _, t := range tags {
		if t.Origin != origin {
			continue
		}
		switch {
		case t.Unreachable:
			_, _ = fmt.Fprintf(w, "    %-14s — no tag reaches this function; check its spelling\n", t.Func)
			wrote = true
		case origin == validate.OriginFlow:
			// No function to name, so these fit on shared lines.
			names = append(names, t.Name)
		case t.Shadows:
			_, _ = fmt.Fprintf(w, "    %-14s   %s, replacing whisk's built-in of the same name\n", t.Name, t.Func)
			wrote = true
		default:
			_, _ = fmt.Fprintf(w, "    %-14s   %s\n", t.Name, t.Func)
			wrote = true
		}
	}
	if len(names) > 0 {
		_, _ = fmt.Fprintf(w, "    %s\n", strings.Join(names, ", "))
		wrote = true
	}
	return wrote
}
