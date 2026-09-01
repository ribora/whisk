package validate_test

import (
	"regexp"
	"strings"
	"testing"

	"gitlab.com/gorib/whisk/internal/plugins/validate"
)

// Kind is a public vocabulary: a consumer switches on it to word its own message.
// A kind emitted as a literal would let whisk and whisk/validate drift apart, so
// every one of them has to arrive as the constant.
func TestKindIsAlwaysTheConstant(t *testing.T) {
	sec := runValidate(t, `package sample

import "time"

type Item struct {
	Name string `+"`validate:\"required\"`"+`
}

type Every struct {
	Str    string    `+"`validate:\"required,min=2\"`"+`
	Num    int       `+"`validate:\"required,gte=1\"`"+`
	Slice  []string  `+"`validate:\"required,min=1,unique\"`"+`
	Nested Item      `+"`validate:\"required\"`"+`
	When   time.Time `+"`validate:\"required,gt=2020-01-01\"`"+`
	Ptr    *Item     `+"`validate:\"required\"`"+`
}
`, validate.Options{})

	code := string(sec.Code)

	if literal := regexp.MustCompile(`Kind:\s+"`).FindString(code); literal != "" {
		t.Errorf("a kind was emitted as a literal:\n%s", code)
	}
	for _, want := range []string{
		"validate.KindStr",
		"validate.KindValue",
		"validate.KindSlice",
		"validate.KindObj",
		"validate.KindTime",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("this model should reach %s, and did not:\n%s", want, code)
		}
	}
}
