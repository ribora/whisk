package err_pointer_embed

// A pointer embed used to be allocated as a side effect of the parent's body
// unmarshal, which broke once the body became conditional. It is refused now,
// and this case keeps the documentation from drifting back.
type Pagination struct {
	Page int `scan:"query=page"`
}

type List struct {
	Page   *Pagination `scan:"embed"`
	Status string      `scan:"query=status"`
}
