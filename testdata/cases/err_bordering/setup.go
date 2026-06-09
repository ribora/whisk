package err_bordering

type Inner struct {
	ID string `scan:"path=id"`
}

// Outer has Inner WITHOUT scan:"embed" — bordering check should error.
type Outer struct {
	Inner Inner
}
