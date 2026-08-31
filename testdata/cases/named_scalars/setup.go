package named_scalars

// Named types over basic kinds: the strconv call is chosen by the underlying
// kind, the conversion names the declared type. No plain scalar fields here on
// purpose — a plain one would pull in the strconv import by itself and hide a
// wrong import decision for the named ones.
type Status string

type Page int

type Ratio float64

type Verbose bool

type Filter struct {
	Status   Status   `scan:"query=status"`
	Page     Page     `scan:"query=page"`
	Ratio    Ratio    `scan:"query=ratio"`
	Verbose  Verbose  `scan:"query=verbose"`
	Statuses []Status `scan:"query=statuses"`
	Pages    []Page   `scan:"query=pages,repeated"`
}
