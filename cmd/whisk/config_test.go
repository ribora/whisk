package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ribora/whisk/internal/plugins/scan"
	"github.com/ribora/whisk/internal/plugins/validate"
)

// No file is the normal case, so it must be indistinguishable from all-defaults.
func TestLoadProjectConfig_absentAndEmpty(t *testing.T) {
	root := t.TempDir()
	cfg, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("no file: %v", err)
	}
	if cfg != (projectConfig{}) {
		t.Errorf("no file yielded %+v, want the zero value", cfg)
	}

	// A file emptied by hand holds no documents; that is not a broken one.
	writeConfig(t, root, "# nothing but comments\n")
	cfg, err = loadProjectConfig(root)
	if err != nil {
		t.Fatalf("comments-only file: %v", err)
	}
	if cfg != (projectConfig{}) {
		t.Errorf("comments-only file yielded %+v, want the zero value", cfg)
	}
}

func TestLoadProjectConfig_sections(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "format: toml\nscan:\n  extractor: internal/formats\nvalidate:\n  storage: internal/checks\n")

	cfg, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("loadProjectConfig: %v", err)
	}
	if cfg.Format != "toml" {
		t.Errorf("format = %q", cfg.Format)
	}
	if cfg.Scan.ExtractorDir != "internal/formats" {
		t.Errorf("scan.extractor = %q", cfg.Scan.ExtractorDir)
	}
	if cfg.Validate.StorageDir != "internal/checks" {
		t.Errorf("validate.storage = %q", cfg.Validate.StorageDir)
	}
}

// A config that silently means something else is worse than no config. The
// wording is asserted too: the reader has sections, not Go types.
func TestLoadProjectConfig_unknownKeyIsAnError(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"top level": {"formats: json\n", "at the top level"},
		"section":   {"validate:\n  store: internal/checks\n", `in the "validate" section`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeConfig(t, root, c.body)

			_, err := loadProjectConfig(root)
			if err == nil {
				t.Fatal("an unknown key was accepted")
			}
			if !strings.Contains(err.Error(), configFile) {
				t.Errorf("error %q does not name the file", err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not say %q", err, c.want)
			}
			if strings.Contains(err.Error(), "in type") {
				t.Errorf("error %q leaks a Go type name", err)
			}
		})
	}
}

// The flag has to win, or a per-file override on a go:generate line could not
// exist.
func TestApplyProjectConfig_precedence(t *testing.T) {
	cfg := projectConfig{
		Format:   "toml",
		Scan:     scan.Config{ExtractorDir: "internal/formats"},
		Validate: validate.Config{StorageDir: "internal/checks"},
	}

	t.Run("file over default", func(t *testing.T) {
		format := "json"
		scanCfg, validateCfg := scan.DefaultConfig(), validate.DefaultConfig()

		applyProjectConfig(cfg, nil, &format, &scanCfg, &validateCfg)

		if format != "toml" || scanCfg.ExtractorDir != "internal/formats" || validateCfg.StorageDir != "internal/checks" {
			t.Errorf("got %q/%q/%q, want the file's values", format, scanCfg.ExtractorDir, validateCfg.StorageDir)
		}
	})

	t.Run("flag over file", func(t *testing.T) {
		format := "xml"
		scanCfg := scan.Config{ExtractorDir: "flagged/scan"}
		validateCfg := validate.Config{StorageDir: "flagged/validate"}
		set := map[string]bool{"format": true, "scan.extractor": true, "validate.storage": true}

		applyProjectConfig(cfg, set, &format, &scanCfg, &validateCfg)

		if format != "xml" || scanCfg.ExtractorDir != "flagged/scan" || validateCfg.StorageDir != "flagged/validate" {
			t.Errorf("got %q/%q/%q, want the flag values", format, scanCfg.ExtractorDir, validateCfg.StorageDir)
		}
	})

	// The file states what it overrides and stays silent about the rest.
	t.Run("empty section keeps the default", func(t *testing.T) {
		format := "json"
		scanCfg, validateCfg := scan.DefaultConfig(), validate.DefaultConfig()

		applyProjectConfig(projectConfig{Format: "xml"}, nil, &format, &scanCfg, &validateCfg)

		if format != "xml" {
			t.Errorf("format = %q, want xml", format)
		}
		if scanCfg.ExtractorDir != "pkg/scan" || validateCfg.StorageDir != "pkg/validate" {
			t.Errorf("got %q/%q, want the defaults", scanCfg.ExtractorDir, validateCfg.StorageDir)
		}
	})
}

// The template writes values, so a drifted one silently pins a project to a path
// whisk no longer uses.
func TestRunInit_templateMatchesTheDefaults(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, configTemplate)

	cfg, err := loadProjectConfig(root)
	if err != nil {
		t.Fatalf("the init template does not load: %v", err)
	}
	want := projectConfig{
		Format:   "json",
		Scan:     scan.DefaultConfig(),
		Validate: validate.DefaultConfig(),
	}
	if cfg != want {
		t.Errorf("the template loads as %+v, want the defaults %+v", cfg, want)
	}
}

func TestRunInit_writesAtTheModuleRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/svc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(root, "domain", "user")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(pkg)

	if err := runInit(); err != nil {
		t.Fatalf("runInit: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, configFile))
	if err != nil {
		t.Fatalf("read %s: %v", configFile, err)
	}
	if string(got) != configTemplate {
		t.Errorf("wrote:\n%s\nwant the template", got)
	}
}

// Edited settings are not the generator's to discard, and a rerun is not a
// failure either — waffle's scaffolding behaves the same way.
func TestRunInit_keepsAnExistingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/svc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const existing = "validate:\n  storage: internal/checks\n"
	writeConfig(t, root, existing)
	t.Chdir(root)

	if err := runInit(); err != nil {
		t.Fatalf("rerunning init reported a failure: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, configFile))
	if err != nil {
		t.Fatalf("read %s: %v", configFile, err)
	}
	if string(got) != existing {
		t.Errorf("file changed to:\n%s", got)
	}
}

func writeConfig(t *testing.T, root, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, configFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
