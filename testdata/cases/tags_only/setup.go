package tags_only

import "github.com/ribora/whisk/scan"

// Inner is body-free: its only field is sourced from a header. Used as
// an embed below to exercise the "scan:\"embed\" alone is not a body
// field" path.
type Inner struct {
	Token string `scan:"header=Authorization" validate:"required"`
}

// FromContext models the mixin pattern: nothing comes from the request
// body, the field is filled by a parse() hook from the request context.
// `scan:"-"` is what keeps it out of bucket 1.
type FromContext struct {
	UserID int `scan:"-" validate:"required"`
}

func (s *FromContext) parse(_ scan.Source) error { return nil }

// Request covers every non-body source kind plus an embed and an opt-out
// field, so it should generate Parse without a body unmarshal block.
type Request struct {
	Inner    Inner       `scan:"embed"`
	Context  FromContext `scan:"embed"`
	FacCode  string      `scan:"path=facilityCode" validate:"required"`
	Page     int         `scan:"query=page"`
	Internal string      `scan:"-"`
}
