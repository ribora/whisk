package shared_test

import (
	"path/filepath"
	"testing"

	"github.com/ribora/whisk/internal/shared"
	"github.com/ribora/whisk/internal/testmodule"
)

// Both predicates decide whether generated code may call a method, so a wrong
// answer is a call to something that is not there — or a second call to something
// that runs already.
func TestValidateMethodAndHookDetection(t *testing.T) {
	dir := testmodule.New(t, `package sample

type Written struct{ X int }

func (w *Written) Validate() error { return nil }

type Hooked struct{ X int }

func (h *Hooked) validate() error { return nil }

type Plain struct{ X int }

type WrongShape struct{ X int }

func (w *WrongShape) Validate() bool { return true }

type Embeds struct {
	Written
	Y int
}
`)

	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]shared.StructInfo{}
	for _, s := range structs {
		byName[s.Name] = s
	}

	cases := []struct {
		name         string
		method, hook bool
	}{
		{"Written", true, false},
		{"Hooked", false, true},
		{"Plain", false, false},
		// Validate() bool is not the shape the generated code calls.
		{"WrongShape", false, false},
		// A value embedding a type with Validate really does carry one.
		{"Embeds", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, ok := byName[c.name]
			if !ok {
				t.Fatalf("struct %s was not loaded", c.name)
			}
			if got := shared.HasOwnValidateMethod(s.Named); got != c.method {
				t.Errorf("HasOwnValidateMethod = %v, want %v", got, c.method)
			}
			if got := shared.HasOwnValidateHook(s.Named); got != c.hook {
				t.Errorf("HasOwnValidateHook = %v, want %v", got, c.hook)
			}
		})
	}
}

// A hook promoted from an embed is already called by the embed's own generated
// code, so the outer type must not report it and call it twice.
func TestPromotedHookIsNotOwn(t *testing.T) {
	dir := testmodule.New(t, `package sample

type Inner struct{ X int }

func (i *Inner) validate() error { return nil }

type Outer struct {
	Inner
	Y int
}
`)

	_, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range structs {
		if s.Name == "Outer" && s.HasValidateHook {
			t.Error("Outer reports the hook it inherited from Inner")
		}
		if s.Name == "Inner" && !s.HasValidateHook {
			t.Error("Inner lost its own hook")
		}
	}
}
