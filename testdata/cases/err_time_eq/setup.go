package err_time_eq

import "time"

// Exact equality against an instant emitted `s.At != 2020-01-01`, which does not
// compile — and would be a strange thing to ask of a request field even if it did.
type Event struct {
	At time.Time `validate:"eq=2020-01-01"`
}
