package shared_test

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/gorib/whisk/internal/shared"
)

func TestLoad_topLevelStructsInSourceOrder(t *testing.T) {
	dir := newModule(t, `package sample

type Z struct {
	ID string `+"`scan:\"path=id\"`"+`
}

type A struct {
	Name string
}

type notExported struct {
	X int
}
`)

	pkgName, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if pkgName != "sample" {
		t.Errorf("pkgName = %q, want sample", pkgName)
	}
	if len(structs) != 3 {
		t.Fatalf("got %d structs, want 3", len(structs))
	}
	gotNames := []string{structs[0].Name, structs[1].Name, structs[2].Name}
	wantNames := []string{"Z", "A", "notExported"}
	for i := range gotNames {
		if gotNames[i] != wantNames[i] {
			t.Errorf("structs[%d].Name = %q, want %q", i, gotNames[i], wantNames[i])
		}
	}
}

func TestLoad_fieldShapes(t *testing.T) {
	dir := newModule(t, `package sample

type Inner struct{ V int }

type S struct {
	Name      string            `+"`validate:\"required\"`"+`
	Tags      []string          `+"`scan:\"query=tag,repeated\"`"+`
	Counts    map[string]int
	Optional  *string
	Inner     Inner
	InnerPtr  *Inner
	Anon      struct{ X int }
	AnonPtr   *struct{ Y int }
	AnonSlice []struct{ Z int }
}
`)

	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var s shared.StructInfo
	for _, st := range structs {
		if st.Name == "S" {
			s = st
		}
	}
	if s.Name == "" {
		t.Fatal("struct S not found")
	}

	byName := map[string]shared.FieldInfo{}
	for _, f := range s.Fields {
		byName[f.Name] = f
	}
	check := func(name string, want shared.FieldInfo) {
		t.Helper()
		got, ok := byName[name]
		if !ok {
			t.Errorf("field %s missing", name)
			return
		}
		if got.Type != want.Type {
			t.Errorf("%s.Type = %q, want %q", name, got.Type, want.Type)
		}
		if got.IsSlice != want.IsSlice {
			t.Errorf("%s.IsSlice = %v, want %v", name, got.IsSlice, want.IsSlice)
		}
		if got.IsMap != want.IsMap {
			t.Errorf("%s.IsMap = %v, want %v", name, got.IsMap, want.IsMap)
		}
		if got.IsPointer != want.IsPointer {
			t.Errorf("%s.IsPointer = %v, want %v", name, got.IsPointer, want.IsPointer)
		}
		if want.ElemType != "" && got.ElemType != want.ElemType {
			t.Errorf("%s.ElemType = %q, want %q", name, got.ElemType, want.ElemType)
		}
		if want.KeyType != "" && got.KeyType != want.KeyType {
			t.Errorf("%s.KeyType = %q, want %q", name, got.KeyType, want.KeyType)
		}
	}

	check("Name", shared.FieldInfo{Type: "string"})
	check("Tags", shared.FieldInfo{Type: "[]string", IsSlice: true, ElemType: "string"})
	check("Counts", shared.FieldInfo{Type: "map[string]int", IsMap: true, KeyType: "string", ElemType: "int"})
	check("Optional", shared.FieldInfo{Type: "*string", IsPointer: true})
	check("Inner", shared.FieldInfo{Type: "Inner"})
	check("InnerPtr", shared.FieldInfo{Type: "*Inner", IsPointer: true})

	if byName["Anon"].AnonymousType == nil {
		t.Error("Anon.AnonymousType nil, want inline struct")
	}
	if !byName["AnonPtr"].IsPointer || byName["AnonPtr"].AnonymousType == nil {
		t.Error("AnonPtr should be pointer-to-inline-struct")
	}
	if !byName["AnonSlice"].IsSlice || byName["AnonSlice"].AnonymousElem == nil {
		t.Error("AnonSlice should expose AnonymousElem")
	}

	if got := byName["Name"].Tag.Get("validate"); got != "required" {
		t.Errorf("Name.Tag validate = %q, want required", got)
	}
	if got := byName["Tags"].Tag.Get("scan"); got != "query=tag,repeated" {
		t.Errorf("Tags.Tag scan = %q, want query=tag,repeated", got)
	}
}

func TestLoad_detectsHooks(t *testing.T) {
	dir := newModule(t, `package sample

import "gitlab.com/gorib/whisk/scan"

type Hooked struct {
	X int
}

func (s *Hooked) validate() error { return nil }
func (s *Hooked) parse(_ scan.Source) error { return nil }

type Plain struct{ Y int }
`)

	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := map[string]shared.StructInfo{}
	for _, st := range structs {
		got[st.Name] = st
	}
	if !got["Hooked"].HasValidateHook {
		t.Error("Hooked.HasValidateHook = false, want true")
	}
	if !got["Hooked"].HasParseHook {
		t.Error("Hooked.HasParseHook = false, want true")
	}
	if got["Plain"].HasValidateHook || got["Plain"].HasParseHook {
		t.Error("Plain should have no hooks")
	}
}

func TestLoad_embeddedField(t *testing.T) {
	dir := newModule(t, `package sample

type Base struct{ ID string }

type Wrapper struct {
	Base
	*Base2
	Name string
}

type Base2 struct{ Y int }
`)

	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var w shared.StructInfo
	for _, st := range structs {
		if st.Name == "Wrapper" {
			w = st
		}
	}
	if w.Name == "" {
		t.Fatal("Wrapper not found")
	}
	if len(w.Fields) != 3 {
		t.Fatalf("Wrapper fields = %d, want 3", len(w.Fields))
	}
	if !w.Fields[0].IsEmbedded || w.Fields[0].IsPointer {
		t.Errorf("Fields[0] (Base) shape wrong: %+v", w.Fields[0])
	}
	if !w.Fields[1].IsEmbedded || !w.Fields[1].IsPointer {
		t.Errorf("Fields[1] (*Base2) shape wrong: %+v", w.Fields[1])
	}
	if w.Fields[2].IsEmbedded {
		t.Errorf("Fields[2] (Name) should not be embedded")
	}
}

// newModule writes a tiny self-contained Go module rooted in t.TempDir().
// A local go.work pins gitlab.com/gorib/whisk to the on-disk checkout so
// tests that import whisk/scan don't need network resolution or go.sum.
func newModule(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	whiskRoot := whiskCheckoutRoot(t)
	files := map[string]string{
		"go.mod":    "module sample\n\ngo 1.25.0\n\nrequire gitlab.com/gorib/whisk v0.0.0\n",
		"go.work":   "go 1.25.0\n\nuse (\n\t.\n\t" + whiskRoot + "\n)\n",
		"sample.go": source,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

// whiskCheckoutRoot returns the absolute path of the whisk module on disk.
// We walk up from this test file (under whisk/internal/shared) two levels.
func whiskCheckoutRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	root, err := filepath.Abs(filepath.Join(cwd, "..", ".."))
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	return root
}
