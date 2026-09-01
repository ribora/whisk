package validate

// Kind reaches the caller inside the error and is what a consumer switches on to
// word its own message. So the generated code names the constant rather than
// repeating its value, and the vocabulary cannot drift apart from whisk/validate.
const (
	kindStr   = "validate.KindStr"
	kindValue = "validate.KindValue"
	kindSlice = "validate.KindSlice"
	kindObj   = "validate.KindObj"
	kindTime  = "validate.KindTime"
)
