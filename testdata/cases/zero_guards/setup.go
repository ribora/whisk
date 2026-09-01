package zero_guards

import "time"

// Every non-required tag sits inside a guard so an absent optional field stays
// quiet, and required's zero check is that guard inverted. Both used to be read
// off the type's spelling, which made a named scalar look like a struct: the
// guard went missing, required emitted nothing at all, and Kind said "obj" for a
// string. time.Time and a hand-written dive were compared against 0.

type ID string

type Page int

// A named scalar can still carry rules of its own, and then both happen.
type Slug string

func (s Slug) Validate() error { return nil }

type Meta struct {
	Name string `validate:"required" json:"name"`
}

type Doc struct {
	Plain    string    `validate:"uuid" json:"plain"`
	ID       ID        `validate:"uuid" json:"id"`
	IDReq    ID        `validate:"required,uuid" json:"id_req"`
	Ptr      *ID       `validate:"uuid" json:"ptr"`
	Page     Page      `validate:"gte=1" json:"page"`
	PageReq  Page      `validate:"required,gte=1" json:"page_req"`
	Slug     Slug      `validate:"min=3" json:"slug"`
	IDs      []ID      `validate:"unique,dive,uuid" json:"ids"`
	Strict   []ID      `validate:"dive,required,uuid" json:"strict"`
	When     time.Time `validate:"gt=2020-01-01" json:"when"`
	WhenReq  time.Time `validate:"required" json:"when_req"`
	Explicit Meta      `validate:"dive" json:"explicit"`
}
