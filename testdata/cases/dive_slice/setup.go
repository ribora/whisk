package dive_slice

type Item struct {
	Name  string `validate:"required,min=1"`
	Price int    `validate:"gt=0"`
}

type Cart struct {
	Items []Item   `validate:"required,min=1"`
	Codes []string `validate:"dive,min=2,max=10"`
}
