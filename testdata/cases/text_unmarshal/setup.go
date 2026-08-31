package text_unmarshal

import "github.com/google/uuid"

// Email has a basic underlying type AND decodes itself from text. The decoder
// wins: it exists to normalise, and converting around it would drop that.
type Email string

func (e *Email) UnmarshalText(b []byte) error {
	*e = Email(b)
	return nil
}

type Request struct {
	ID       uuid.UUID   `scan:"path=id"`
	Parent   *uuid.UUID  `scan:"query=parent"`
	Related  []uuid.UUID `scan:"query=related"`
	Assignee []uuid.UUID `scan:"query=assignee,repeated"`
	Contact  Email       `scan:"query=contact"`
	// A slice of pointers would need an allocation per element; the marker is
	// the honest answer rather than a nil receiver.
	Watchers []*uuid.UUID `scan:"query=watcher,repeated"`
}
