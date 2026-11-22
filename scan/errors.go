package scan

import "github.com/ribora/failure"

// ErrPayload is returned when the request cannot be decoded.
var ErrPayload = failure.New("invalid payload")
