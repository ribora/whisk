// whisk generates Parse(src scan.Source) error and Validate() error
// methods for structs declared in the target file.
//
//	//go:generate whisk $GOFILE
//	//go:generate whisk -format=xml $GOFILE
//	//go:generate whisk -validate.storage=internal/checks $GOFILE
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/gorib/whisk/internal/generator"
	"gitlab.com/gorib/whisk/internal/plugins/scan"
	"gitlab.com/gorib/whisk/internal/plugins/validate"
)

type flagBinder interface {
	BindFlags(fs *flag.FlagSet, prefix string)
}

func main() {
	// -format is cross-cutting (scan's decoder + validate's path tags), so it
	// stays top-level rather than under a -<plugin>. namespace.
	formatName := flag.String("format", "json", "body format: json | xml")

	// Plugin-scoped flags register under the plugin's name. Add swagger etc.
	// to this table as they gain flags.
	scanCfg := scan.DefaultConfig()
	validateCfg := validate.DefaultConfig()
	binders := map[string]flagBinder{
		"scan":     &scanCfg,
		"validate": &validateCfg,
	}
	for prefix, b := range binders {
		b.BindFlags(flag.CommandLine, prefix)
	}

	flag.Parse()

	target, err := pickTarget(flag.Args())
	if err != nil {
		die("%v", err)
	}

	pkgDir := filepath.Dir(target)
	if pkgDir == "" {
		pkgDir = "."
	}
	absPkgDir, err := filepath.Abs(pkgDir)
	if err != nil {
		die("resolve package dir: %v", err)
	}
	root, err := findProjectRoot(absPkgDir)
	if err != nil {
		die("%v", err)
	}
	module, err := readModuleName(filepath.Join(root, "go.mod"))
	if err != nil {
		die("%v", err)
	}

	extractorAbs, extractorImport := resolveDir(root, module, scanCfg.ExtractorDir)
	format, err := scan.ResolveFormat(*formatName, extractorAbs, extractorImport)
	if err != nil {
		die("%v", err)
	}
	storageAbs, userChecksImport := resolveDir(root, module, validateCfg.StorageDir)

	plugins := []generator.Plugin{
		scan.New(format),
		validate.New(validate.Options{
			StoragePath:      storageAbs,
			UserChecksImport: userChecksImport,
			// Path segments use the body format's tag names (json/xml/custom).
			ExtractTags: []string{format.Tag},
		}),
	}

	if err := generator.Generate(target, plugins); err != nil {
		die("%v", err)
	}
}

func pickTarget(args []string) (string, error) {
	if len(args) > 1 {
		return "", fmt.Errorf("expected at most one file, got %d", len(args))
	}
	if len(args) == 1 {
		return args[0], nil
	}
	if env := os.Getenv("GOFILE"); env != "" {
		return env, nil
	}
	return "", fmt.Errorf("no input file (set $GOFILE or pass a path)")
}

// The import path comes back even when the directory is absent: a plugin uses it
// only once it has found something there.
func resolveDir(root, module, rel string) (abs, importPath string) {
	abs = filepath.Join(root, rel)
	r, err := filepath.Rel(root, abs)
	if err != nil || r == "." {
		return abs, module
	}
	return abs, module + "/" + filepath.ToSlash(r)
}

func findProjectRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above %s", start)
		}
		dir = parent
	}
}

func readModuleName(goModPath string) (string, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("no module directive in %s", goModPath)
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "whisk: "+format+"\n", args...)
	os.Exit(1)
}
