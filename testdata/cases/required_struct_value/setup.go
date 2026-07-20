package required_struct_value

// Meta is a plain struct value with no validate tags — nothing to validate.
type Meta struct {
	Source string `json:"source"`
}

// Doc carries `required` on a non-pointer struct value. required has no zero
// check for a struct value, so it emits no validate.* code: Doc still gets a
// Validate (the tag makes it "contribute"), but the body is empty and the
// whisk/validate import must NOT be added — otherwise the file won't compile.
type Doc struct {
	Id   int  `json:"id"`
	Meta Meta `json:"meta" validate:"required"`
}
