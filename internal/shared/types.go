package shared

import (
	"go/types"
	"reflect"
)

// Every struct in the file, unfiltered: what to emit is each plugin's decision.
type StructInfo struct {
	Name    string
	PkgPath string
	Fields  []FieldInfo
	// Own hooks only, or an embed's hook would be called twice.
	HasValidateHook bool
	HasParseHook    bool
	// For plugins that walk the type graph rather than the field list.
	Named *types.Named
}

type FieldInfo struct {
	Name string
	Tag  reflect.StructTag
	// Qualified against the struct's own package: "Inner", not "model.Inner".
	Type    string
	TypeRef types.Type

	IsPointer bool
	// IsSlice covers arrays too, since ranging over them is the same. IsArray
	// separates the case where it is not: an array cannot be built with make and
	// append, so anything that writes a collection has to ask.
	IsSlice    bool
	IsArray    bool
	IsMap      bool
	IsEmbedded bool

	// Empty unless the field is a slice/array/map.
	ElemType string
	KeyType  string

	// Set only for an inline struct literal, as the field or as its element.
	AnonymousType *types.Struct
	AnonymousElem *types.Struct
}
