package scoped_path

import (
	"errors"
	"slices"
	"testing"

	"github.com/ribora/whisk/validate"
)

// A scoped field is named by its scan key unless the format tag names it.
func TestScopedFieldIsNamedByItsKey(t *testing.T) {
	err := (&PriceQuery{}).Validate()
	var got []string
	for _, e := range err.(interface{ Unwrap() []error }).Unwrap() {
		var ve *validate.ValidationError
		if errors.As(e, &ve) {
			got = append(got, ve.Segments.Format(validate.JSONFormatter))
		}
	}
	if want := []string{"startedAt", "kind"}; !slices.Equal(got, want) {
		t.Errorf("paths = %q, want %q", got, want)
	}
}
