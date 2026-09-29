package embed_body

import "time"

type Actor struct {
	ID string `scan:"header=X-User-Id"`
}

type Note struct {
	Text string `json:"note"`
}

type Audit struct {
	Reason string `json:"reason"`
}

type UpdateUser struct {
	Actor Actor `scan:"embed"`
	Note  Note  `scan:"embed"`
	Audit
	Name  string    `json:"name"`
	Until time.Time `json:"until"`
	Limit int       `json:"limit"`
}
