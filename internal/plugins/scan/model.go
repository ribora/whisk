package scan

type lexem struct {
	Name          string
	Delegates     []delegate
	Fields        []sourceField
	HasHook       bool // *T defines parse(scan.Source) error
	HasBodyFields bool
	Shape         []shapeField
	ShapeRefs     []string
}

type shapeField struct {
	Name     string
	Type     string
	Tag      string
	Embedded bool
}

type delegate struct {
	Name string
}

type sourceField struct {
	Name  string
	Scope string
	Key   string
	// type Page int -> Type "Page", Kind "int": parsed by Kind, stored as Type
	Type     string
	Kind     string
	ElemType string // populated for []T fields
	ElemKind string
	Repeated bool // true ⇒ emit src.LookupAll, false ⇒ comma-split src.Lookup
	// Text wins over Kind: a type that declares UnmarshalText means to be built
	// through it, and converting around it drops whatever it normalises.
	Text     bool
	Pointee  string
	ElemText bool
	// Only types the emitter spells out: an unused import is a compile error, so
	// a superset here breaks as surely as a missing one.
	Refs []string
}
