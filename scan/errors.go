package scan

import "gitlab.com/gorib/failure"

// ErrPayload is returned when the request cannot be decoded.
var ErrPayload = failure.New("invalid payload")
