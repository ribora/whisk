package hook_only

import "github.com/ribora/whisk/scan"

// Hooked has no scan or validate tags, but defines both hooks. The
// generated Parse must still emit body unmarshal + a call to s.parse(src),
// and Validate must call s.validate() before returning.
type Hooked struct {
	Name string
}

func (s *Hooked) parse(_ scan.Source) error { return nil }
func (s *Hooked) validate() error           { return nil }
