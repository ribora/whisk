package dive_map

// The map cases are here because they were once wrong: a map of structs reported
// its map and not the entry, so "which one failed" had no answer. Slices never had
// that problem, which is exactly why nothing caught it.

type Item struct {
	Name  string `validate:"required,min=1" json:"name"`
	Price int    `validate:"gt=0" json:"price"`
}

type Catalogue struct {
	Items    map[string]Item   `validate:"dive" json:"items"`
	Labels   map[string]string `validate:"dive,min=2,max=10" json:"labels"`
	Featured []Item            `validate:"dive" json:"featured"`
}
