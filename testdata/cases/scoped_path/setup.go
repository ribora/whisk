package scoped_path

type PriceQuery struct {
	StartDate string `scan:"query=startedAt" validate:"required"`
	Kind      string `json:"kind" scan:"query=productKind" validate:"required"`
}
