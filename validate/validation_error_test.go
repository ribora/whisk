package validate_test

import (
	"errors"
	"testing"

	"gitlab.com/gorib/whisk/validate"
)

func TestPathFormatting(t *testing.T) {
	p := validate.Path{
		validate.NewFieldSegment("go", "Order", "json", "order"),
		validate.NewFieldSegment("go", "Items", "json", "items"),
		validate.NewIndexSegment(3),
		validate.NewFieldSegment("go", "Name", "json", "name"),
	}
	if got, want := p.Format(validate.GoFormatter), "Order.Items[3].Name"; got != want {
		t.Errorf("GoFormatter = %q, want %q", got, want)
	}
	if got, want := p.Format(validate.JSONFormatter), "order.items[3].name"; got != want {
		t.Errorf("JSONFormatter = %q, want %q", got, want)
	}
	if got, want := p.String(), "Order.Items[3].Name"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestKeySegment(t *testing.T) {
	p := validate.Path{
		validate.NewFieldSegment("go", "Rates"),
		validate.NewKeySegment("USD"),
		validate.NewFieldSegment("go", "Amount"),
	}
	if got, want := p.String(), "Rates[USD].Amount"; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	// The key is whatever the map is keyed by, so it arrives through fmt.
	intKey := validate.Path{validate.NewKeySegment(7)}
	if got, want := intKey.String(), "[7]"; got != want {
		t.Errorf("int key = %q, want %q", got, want)
	}
}

// A tag the field does not carry falls back to the Go name, which is why a
// half-tagged struct still renders a complete path.
func TestNameFallsBackToGo(t *testing.T) {
	seg := validate.NewFieldSegment("go", "DisplayName", "json", "display_name")
	if got, want := seg.GetName("xml"), "DisplayName"; got != want {
		t.Errorf("GetName(xml) = %q, want %q", got, want)
	}
	if got, want := seg.GetName("json"), "display_name"; got != want {
		t.Errorf("GetName(json) = %q, want %q", got, want)
	}
	if got := (validate.NewFieldSegment("json", "only")).GetName("go"); got != "" {
		t.Errorf("with no go entry GetName(go) = %q, want empty", got)
	}
}

// The generator always emits pairs. An odd tail is dropped rather than paired
// with an empty string, so a miscount loses a name instead of inventing one.
func TestFieldSegmentDropsAnOddTail(t *testing.T) {
	seg := validate.NewFieldSegment("go", "Name", "json")
	if got, want := len(seg.Names), 1; got != want {
		t.Fatalf("len(Names) = %d, want %d", got, want)
	}
	if got := seg.GetName("json"); got != "Name" {
		t.Errorf("GetName(json) = %q, want the go fallback", got)
	}
}

func TestErrorMessage(t *testing.T) {
	seg := validate.NewFieldSegment("go", "Token")
	cases := []struct {
		label string
		err   *validate.ValidationError
		want  string
	}{
		{
			label: "tag only",
			err:   &validate.ValidationError{Segments: validate.Path{seg}, Tag: "required", Kind: validate.KindStr},
			want:  "field 'Token' failed on 'required' (str)",
		},
		{
			label: "with argument",
			err:   &validate.ValidationError{Segments: validate.Path{seg}, Tag: "min", Value: "8", Kind: validate.KindStr},
			want:  "field 'Token' failed on 'min=8' (str)",
		},
		{
			label: "with a reason",
			err: &validate.ValidationError{
				Segments: validate.Path{seg}, Tag: "email", Kind: validate.KindStr,
				Err: errors.New("mail: no address"),
			},
			want: "field 'Token' failed on 'email' (str): mail: no address",
		},
		{
			label: "no kind",
			err:   &validate.ValidationError{Segments: validate.Path{seg}, Tag: "required"},
			want:  "field 'Token' failed on 'required'",
		},
	}
	for _, c := range cases {
		if got := c.err.Error(); got != c.want {
			t.Errorf("%s: Error() = %q, want %q", c.label, got, c.want)
		}
	}
}

func TestUnwrapReachesTheReason(t *testing.T) {
	reason := errors.New("mail: no address")
	err := &validate.ValidationError{Tag: "email", Err: reason}
	if !errors.Is(err, reason) {
		t.Error("errors.Is should reach the underlying reason")
	}

	var target *validate.ValidationError
	if !errors.As(errors.Join(errors.New("other"), err), &target) || target.Tag != "email" {
		t.Error("errors.As should find the ValidationError inside a joined error")
	}
}

func TestPrependSegment(t *testing.T) {
	err := &validate.ValidationError{
		Segments: validate.Path{validate.NewFieldSegment("go", "Name")},
		Tag:      "required",
	}
	err.PrependSegment(validate.NewFieldSegment("go", "Items"), validate.NewIndexSegment(2))
	if got, want := err.Segments.String(), "Items[2].Name"; got != want {
		t.Errorf("Segments = %q, want %q", got, want)
	}
}

// The generated code records how many errors it had before descending, then
// prefixes only what the nested call added.
func TestPrependFromLeavesEarlierErrorsAlone(t *testing.T) {
	own := &validate.ValidationError{Segments: validate.Path{validate.NewFieldSegment("go", "Own")}}
	nested := &validate.ValidationError{Segments: validate.Path{validate.NewFieldSegment("go", "Inner")}}
	errs := []error{own, errors.New("not a validation error"), nested}

	validate.PrependFrom(errs, 1, validate.NewFieldSegment("go", "Outer"))

	if got, want := own.Segments.String(), "Own"; got != want {
		t.Errorf("error before the index changed: %q, want %q", got, want)
	}
	if got, want := nested.Segments.String(), "Outer.Inner"; got != want {
		t.Errorf("nested = %q, want %q", got, want)
	}
}

func TestAppendSubErrorsFlattensAJoin(t *testing.T) {
	a := &validate.ValidationError{Segments: validate.Path{validate.NewFieldSegment("go", "A")}, Tag: "required"}
	b := &validate.ValidationError{Segments: validate.Path{validate.NewFieldSegment("go", "B")}, Tag: "required"}
	plain := errors.New("from a hook")

	var errs []error
	validate.AppendSubErrors(&errs, errors.Join(a, plain, b), validate.NewFieldSegment("go", "Nested"))

	if got, want := len(errs), 3; got != want {
		t.Fatalf("len(errs) = %d, want %d — the join should be flattened, not nested", got, want)
	}
	if got, want := a.Segments.String(), "Nested.A"; got != want {
		t.Errorf("a = %q, want %q", got, want)
	}
	if got, want := b.Segments.String(), "Nested.B"; got != want {
		t.Errorf("b = %q, want %q", got, want)
	}
	// A foreign error keeps its place in the list; there is no path to prefix.
	if errs[1] != plain {
		t.Errorf("errs[1] = %v, want the hook's own error", errs[1])
	}
}

func TestAppendSubErrorsTakesASingleError(t *testing.T) {
	single := &validate.ValidationError{Segments: validate.Path{validate.NewFieldSegment("go", "Field")}}
	var errs []error
	validate.AppendSubErrors(&errs, single, validate.NewFieldSegment("go", "Outer"))

	if got, want := len(errs), 1; got != want {
		t.Fatalf("len(errs) = %d, want %d", got, want)
	}
	if got, want := single.Segments.String(), "Outer.Field"; got != want {
		t.Errorf("Segments = %q, want %q", got, want)
	}
}

func TestFormatterForUnknownKeyIsGo(t *testing.T) {
	f := validate.FormatterFor("toml")
	seg := validate.NewFieldSegment("go", "Field", "json", "field")
	if got, want := f(0, seg), "Field"; got != want {
		t.Errorf("FormatterFor(toml) = %q, want %q", got, want)
	}
	if got, want := f(1, seg), ".Field"; got != want {
		t.Errorf("non-leading segment = %q, want %q", got, want)
	}
}
