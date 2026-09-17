package narrow_scalars

// The strconv bit size used to come from the declared name, so a named type fell
// through to 64 and strconv accepted a value the conversion back to the field's
// width silently truncated: ?small=300 landed as 44, ?ratio=1e300 as +Inf.
//
// named_scalars covers string/int/float64/bool — the four widths where that does
// not show. These are the ones where it does, plain beside named so the golden
// shows the pair.

type Small int8

type Port uint16

type Ratio float32

type Filter struct {
	PlainSmall int8    `scan:"query=plain_small"`
	Small      Small   `scan:"query=small"`
	PlainPort  uint16  `scan:"query=plain_port"`
	Port       Port    `scan:"query=port"`
	PlainRatio float32 `scan:"query=plain_ratio"`
	Ratio      Ratio   `scan:"query=ratio"`
	Smalls     []Small `scan:"query=smalls,repeated"`
	Count      int     `scan:"query=count"`
}
