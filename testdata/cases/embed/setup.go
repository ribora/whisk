package embed

type Auth struct {
	Token string `scan:"header=Authorization" validate:"required"`
}

type Pagination struct {
	Page    int `scan:"query=page" validate:"gte=1"`
	PerPage int `scan:"query=per_page" validate:"gte=1,lte=100"`
}

type ListRequest struct {
	Auth       Auth        `scan:"embed"`
	Pagination *Pagination `scan:"embed"`
	UserID     string      `scan:"path=userId" validate:"required"`
}
