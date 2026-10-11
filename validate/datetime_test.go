package validate_test

import (
	"testing"
	"time"

	"gitlab.com/gorib/whisk/validate"
)

func TestIsDatetime(t *testing.T) {
	cases := []struct {
		value string
		args  []string
		want  bool
	}{
		{value: "2020-01-01T00:00:00Z", want: true},
		{value: "2020-01-01T00:00:00+03:00", want: true},
		// RFC3339 is the default, so a bare date needs the format spelled out.
		{value: "2020-01-01", want: false},
		{value: "2020-01-01", args: []string{"2006-01-02"}, want: true},
		{value: "01/02/2020", args: []string{"01/02/2006"}, want: true},
		{value: "2020-13-01", args: []string{"2006-01-02"}, want: false},
		{value: "", want: false},
	}
	for _, c := range cases {
		if got := validate.IsDatetime(c.value, c.args...); got != c.want {
			t.Errorf("IsDatetime(%q, %v) = %v, want %v", c.value, c.args, got, c.want)
		}
	}
}

func local(t *testing.T, value string) string {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02T15:04:05", value, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Format(time.RFC3339)
}

// A date-only bound covers the whole day: min opens it at 00:00:00 and max
// closes it at 23:59:59.999999999, so a timestamp inside the day satisfies both.
func TestDateOnlyBoundsSpanTheWholeDay(t *testing.T) {
	day := "2020-06-15"
	cases := []struct {
		value    string
		min, max bool
	}{
		{value: local(t, "2020-06-15T00:00:00"), min: true, max: true},
		{value: local(t, "2020-06-15T23:59:59"), min: true, max: true},
		{value: local(t, "2020-06-14T23:59:59"), min: false, max: true},
		{value: local(t, "2020-06-16T00:00:00"), min: true, max: false},
	}
	for _, c := range cases {
		if got := validate.IsTimeMin(c.value, day); got != c.min {
			t.Errorf("IsTimeMin(%q, %q) = %v, want %v", c.value, day, got, c.min)
		}
		if got := validate.IsTimeMax(c.value, day); got != c.max {
			t.Errorf("IsTimeMax(%q, %q) = %v, want %v", c.value, day, got, c.max)
		}
	}
}

// A bound that names an instant is used as an instant, so min/max become plain
// inclusive comparisons with no widening to the day.
func TestInstantBoundsAreNotWidened(t *testing.T) {
	bound := "2020-06-15T12:00:00Z"
	cases := []struct {
		value    string
		min, max bool
	}{
		{value: "2020-06-15T12:00:00Z", min: true, max: true},
		{value: "2020-06-15T11:59:59Z", min: false, max: true},
		{value: "2020-06-15T12:00:01Z", min: true, max: false},
	}
	for _, c := range cases {
		if got := validate.IsTimeMin(c.value, bound); got != c.min {
			t.Errorf("IsTimeMin(%q, %q) = %v, want %v", c.value, bound, got, c.min)
		}
		if got := validate.IsTimeMax(c.value, bound); got != c.max {
			t.Errorf("IsTimeMax(%q, %q) = %v, want %v", c.value, bound, got, c.max)
		}
	}
}

// gt/lt take the bound literally instead, so midnight of the bound day fails a
// gt against that same day — the difference from min/max is deliberate.
func TestStrictBoundsUseTheInstant(t *testing.T) {
	if validate.IsTimeGT(local(t, "2020-06-15T00:00:00"), "2020-06-15") {
		t.Error("IsTimeGT at the bound instant should be false")
	}
	if !validate.IsTimeGT(local(t, "2020-06-15T00:00:01"), "2020-06-15") {
		t.Error("IsTimeGT one second past the bound should be true")
	}
	if validate.IsTimeLT(local(t, "2020-06-15T00:00:00"), "2020-06-15") {
		t.Error("IsTimeLT at the bound instant should be false")
	}
	if !validate.IsTimeLT(local(t, "2020-06-14T23:59:59"), "2020-06-15") {
		t.Error("IsTimeLT before the bound should be true")
	}
}

func TestDateOnlyValueIsALocalDay(t *testing.T) {
	saved := time.Local
	t.Cleanup(func() { time.Local = saved })
	time.Local = time.FixedZone("UTC-12", -12*60*60)

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	cases := []struct {
		value, bound string
		min, max     bool
	}{
		{value: today, bound: "now", min: true, max: true},
		{value: yesterday, bound: "now", min: false, max: true},
		{value: today, bound: "0d", min: true, max: true},
	}
	for _, c := range cases {
		if got := validate.IsTimeMin(c.value, c.bound); got != c.min {
			t.Errorf("IsTimeMin(%q, %q) = %v, want %v", c.value, c.bound, got, c.min)
		}
		if got := validate.IsTimeMax(c.value, c.bound); got != c.max {
			t.Errorf("IsTimeMax(%q, %q) = %v, want %v", c.value, c.bound, got, c.max)
		}
	}
}

func TestTimeValueShapes(t *testing.T) {
	ts := time.Date(2020, 6, 15, 12, 0, 0, 0, time.UTC)
	if !validate.IsTimeGT(ts, "2020-06-14") {
		t.Error("time.Time value not accepted")
	}
	if !validate.IsTimeGT(&ts, "2020-06-14") {
		t.Error("*time.Time not accepted")
	}
	// Everything unparseable answers false rather than raising: a nil pointer, a
	// wrong type, an unreadable value and an unreadable bound all land here.
	for _, c := range []struct {
		label  string
		value  any
		target string
	}{
		{"nil pointer", (*time.Time)(nil), "2020-06-14"},
		{"wrong type", 42, "2020-06-14"},
		{"unparseable value", "yesterday", "2020-06-14"},
		{"unparseable bound", ts, "yesterday"},
	} {
		if validate.IsTimeGT(c.value, c.target) {
			t.Errorf("IsTimeGT with %s should be false", c.label)
		}
		if validate.IsTimeLT(c.value, c.target) {
			t.Errorf("IsTimeLT with %s should be false", c.label)
		}
	}
}

func TestRelativeBounds(t *testing.T) {
	now := time.Now().UTC()
	past := now.AddDate(0, -1, 0).Format(time.RFC3339)
	future := now.AddDate(0, 1, 0).Format(time.RFC3339)

	cases := []struct {
		value, target string
		gt, lt        bool
	}{
		{value: past, target: "now", gt: false, lt: true},
		{value: future, target: "now", gt: true, lt: false},
		// A month out is past every one of these windows, and a month back is
		// short of all of them.
		{value: future, target: "7d", gt: true, lt: false},
		{value: future, target: "7days", gt: true, lt: false},
		{value: past, target: "-7d", gt: false, lt: true},
		{value: future, target: "24h", gt: true, lt: false},
		{value: past, target: "5m", gt: false, lt: true},
	}
	for _, c := range cases {
		if got := validate.IsTimeGT(c.value, c.target); got != c.gt {
			t.Errorf("IsTimeGT(%q, %q) = %v, want %v", c.value, c.target, got, c.gt)
		}
		if got := validate.IsTimeLT(c.value, c.target); got != c.lt {
			t.Errorf("IsTimeLT(%q, %q) = %v, want %v", c.value, c.target, got, c.lt)
		}
	}
}

// "2h" is a bound, "2x" is not a unit the parser knows — it falls through to the
// absolute formats, fails those too, and the comparison reports false.
func TestUnknownRelativeUnitFails(t *testing.T) {
	if validate.IsTimeGT(time.Now().Add(time.Hour), "2x") {
		t.Error("an unparseable bound should not pass")
	}
	if validate.IsTimeMin(time.Now().Add(time.Hour), "2x") {
		t.Error("an unparseable bound should not pass")
	}
}
