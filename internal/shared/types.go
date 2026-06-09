package shared

import (
	"go/types"
	"reflect"
)

// StructInfo describes a top-level struct in the target file. The parser
// returns every struct unfiltered; deciding what to emit is each plugin's job.
type StructInfo struct {
	Name    string
	PkgPath string
	Fields  []FieldInfo
	// Own hooks only — ones promoted from an embed are excluded, so the outer
	// type doesn't double-call them.
	HasValidateHook bool
	HasParseHook    bool
	// Named is the struct's type, for plugins that walk the type graph
	// (e.g. validate's reachability check for nested validation).
	Named *types.Named
}

type FieldInfo struct {
	Name string
	Tag  reflect.StructTag
	// Type qualified against the struct's own package, so same-package named
	// types are unqualified (e.g. "Inner", not "model.Inner").
	Type    string
	TypeRef types.Type

	IsPointer  bool
	IsSlice    bool
	IsMap      bool
	IsEmbedded bool

	// Empty unless the field is a slice/array/map.
	ElemType string
	KeyType  string

	// Non-nil only when the field's type is an inline struct literal (Type)
	// or such a slice/array/map element (Elem).
	AnonymousType *types.Struct
	AnonymousElem *types.Struct
}
