package scoped_body

type Request struct {
	UserID string `scan:"header=X-User-Id"`
	Page   int    `scan:"query=page"`
	Name   string `json:"name"`
	Note   string `scan:"-"`
}
