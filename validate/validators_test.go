package validate_test

import (
	"net/url"
	"testing"

	"github.com/ribora/whisk/validate"
)

// Every built-in reduced to one shape, so the tables below read the same whether
// the validator reports a reason or just a verdict.
type check func(string, ...string) (bool, error)

func verdict(f func(string, ...string) bool) check {
	return func(v string, args ...string) (bool, error) { return f(v, args...), nil }
}

type tc struct {
	value   string
	args    []string
	want    bool
	wantErr bool
	why     string
}

func run(t *testing.T, name string, f check, cases []tc) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		for _, c := range cases {
			got, err := f(c.value, c.args...)
			if got != c.want || (err != nil) != c.wantErr {
				t.Errorf("%s(%q, %v) = (%v, %v), want (%v, err:%v)%s",
					name, c.value, c.args, got, err, c.want, c.wantErr, note(c.why))
			}
		}
	})
}

func note(why string) string {
	if why == "" {
		return ""
	}
	return " — " + why
}

func TestStringValidators(t *testing.T) {
	run(t, "email", validate.IsEmail[string], []tc{
		{value: "a@b.com", want: true},
		{value: "not-an-email", wantErr: true},
		{value: "", wantErr: true},
		{value: "a@b.com, c@d.com", wantErr: true, why: "one address per field"},
		// net/mail is the parser, so a display name and a bracketed host parse
		// too. Anything reading the field as a bare address has to cope.
		{value: "John Doe <j@d.com>", want: true},
		{value: "  a@b.com  ", want: true},
		{value: "a@b", want: true, why: "no TLD required"},
		{value: "a@[192.168.0.1]", want: true},
		{value: "a@b.com", args: []string{"smtp"}, wantErr: true, why: "unknown argument"},
	})

	run(t, "uuid", validate.IsUuid[string], []tc{
		{value: "0191b2a0-0000-7000-8000-000000000000", want: true},
		{value: "0191b2a0-0000-7000-8000-000000000000", args: []string{"7"}, want: true},
		{value: "0191b2a0-0000-7000-8000-000000000000", args: []string{"4"}, wantErr: true},
		{value: "not-a-uuid", wantErr: true},
		{value: "", wantErr: true},
		{value: "0191b2a0-0000-7000-8000-000000000000", args: []string{"x"}, wantErr: true},
	})

	run(t, "url", verdict(validate.IsUrl[string]), []tc{
		{value: "http://a.com", want: true},
		{value: "https://a.com/x?y=1", want: true},
		{value: "not a url", want: false},
		{value: "", want: false},
		{value: "a.com", want: false, why: "a bare host is not a request URI"},
		// ParseRequestURI takes any absolute path or scheme, so these pass.
		{value: "/path", want: true},
		{value: "//a.com", want: true},
		{value: "mailto:a@b.com", want: true},
	})

	run(t, "charset", validate.IsCharset, []tc{
		{value: "abc", args: []string{"alpha"}, want: true},
		{value: "абв", args: []string{"alpha"}, want: true, why: "unicode letters, not ASCII"},
		{value: "ab1", args: []string{"alpha"}, want: false},
		{value: "ab1", args: []string{"alphanum"}, want: true},
		{value: "ab 1", args: []string{"alphanum"}, want: false},
		{value: "123", args: []string{"numeric"}, want: true},
		{value: "12.3", args: []string{"numeric"}, want: false},
		{value: "deadBEEF", args: []string{"hex"}, want: true},
		{value: "deadbeeg", args: []string{"hex"}, want: false},
		{value: "", args: []string{"alpha"}, want: false, why: "empty is not a charset match"},
		{value: "abc", wantErr: true, why: "the set is mandatory"},
		{value: "abc", args: []string{"letters"}, wantErr: true},
	})

	run(t, "ip", validate.IsIp, []tc{
		{value: "192.0.2.1", want: true},
		{value: "::1", want: true},
		{value: "1.2.3", want: false},
		{value: "", want: false},
		{value: "192.0.2.1", args: []string{"v4"}, want: true},
		{value: "::1", args: []string{"v6"}, want: true},
		{value: "192.0.2.1", args: []string{"v6"}, want: false},
		{value: "::1", args: []string{"v4"}, want: false},
		// The IPv4-mapped form is written as v6 and counts as v6.
		{value: "::ffff:192.0.2.1", args: []string{"v6"}, want: true},
		{value: "::ffff:192.0.2.1", args: []string{"v4"}, want: false},
		{value: "fe80::1%eth0", want: false, why: "a scoped address is host-specific"},
		{value: "192.0.2.1", args: []string{"v5"}, wantErr: true},
	})

	run(t, "cidr", validate.IsCidr, []tc{
		{value: "192.0.2.0/24", want: true},
		{value: "2001:db8::/32", want: true},
		{value: "192.0.2.1", wantErr: true, why: "an address is not a block"},
		{value: "192.0.2.0/33", wantErr: true},
		{value: "", wantErr: true},
	})

	run(t, "mac", validate.IsMac, []tc{
		{value: "00:1b:63:84:45:e6", want: true},
		{value: "00-1b-63-84-45-e6", want: true},
		{value: "001b.6384.45e6", want: true},
		{value: "00:1b:63:84:45", wantErr: true},
		{value: "", wantErr: true},
	})

	run(t, "base64", validate.IsBase64, []tc{
		{value: "YWJj", want: true},
		{value: "YWJjZA==", want: true},
		{value: "abc", wantErr: true, why: "length is not a multiple of four"},
		{value: "YWJj=", wantErr: true},
		{value: "a-b_c", wantErr: true, why: "std alphabet, not URL-safe"},
		// Every other validator here calls the empty string invalid.
		{value: "", want: true},
	})

	run(t, "json", verdict(validate.IsJson), []tc{
		{value: `{"a":1}`, want: true},
		{value: `[1,2]`, want: true},
		{value: `1`, want: true, why: "a scalar is a document"},
		{value: `{`, want: false},
		{value: ``, want: false},
	})

	run(t, "duration", validate.IsDuration, []tc{
		{value: "1h30m", want: true},
		{value: "500ms", want: true},
		{value: "-2s", want: true},
		{value: "1", wantErr: true, why: "a unit is required"},
		{value: "1 hour", wantErr: true},
		{value: "", wantErr: true},
	})

	run(t, "timezone", validate.IsTimezone, []tc{
		{value: "UTC", want: true},
		{value: "Europe/Moscow", want: true},
		{value: "Local", want: false, why: "host-specific, refused on purpose"},
		{value: "", want: false},
		{value: "Nowhere/Nowhere", wantErr: true},
	})

	run(t, "semver", verdict(validate.IsSemver), []tc{
		{value: "1.2.3", want: true},
		{value: "1.2.3-rc.1+build.5", want: true},
		{value: "0.0.0", want: true},
		{value: "v1.2.3", want: false, why: "no v prefix"},
		{value: "1.2", want: false},
		{value: "01.2.3", want: false, why: "no leading zeroes"},
		{value: "", want: false},
	})

	run(t, "hexcolor", verdict(validate.IsHexcolor), []tc{
		{value: "#fff", want: true},
		{value: "#ffff", want: true},
		{value: "#ffffff", want: true},
		{value: "#ffffffff", want: true},
		{value: "fff", want: false, why: "the hash is required"},
		{value: "#fffff", want: false},
		{value: "#ggg", want: false},
	})

	run(t, "hostname", verdict(validate.IsHostname), []tc{
		{value: "a.b", want: true},
		{value: "localhost", want: true},
		{value: "EXAMPLE.com", want: true, why: "case-insensitive"},
		{value: "a-b.c", want: true},
		{value: "-a.b", want: false},
		{value: "a-.b", want: false},
		{value: "example.com.", want: false, why: "the root dot is not accepted"},
		{value: "", want: false},
	})

	run(t, "fqdn", verdict(validate.IsFqdn), []tc{
		{value: "example.com", want: true},
		{value: "localhost", want: false, why: "a dot is what separates it from a hostname"},
		{value: "-a.b", want: false},
	})

	run(t, "phone", verdict(validate.IsPhone), []tc{
		{value: "+12025550123", want: true},
		{value: "+12025550123", args: []string{"e164"}, want: true},
		{value: "+12025550123", args: []string{""}, want: true},
		{value: "12025550123", want: false, why: "the plus is part of E.164"},
		{value: "+02025550123", want: false, why: "the country code cannot start with zero"},
		{value: "+1 202 555 0123", want: false},
		{value: "", want: false},
		// ip and charset raise an error on an unknown argument. phone declares
		// no error in the registry, so a typo just fails every value.
		{value: "+12025550123", args: []string{"nanp"}, want: false},
	})

	run(t, "lowercase", verdict(validate.IsLowercase), []tc{
		{value: "abc", want: true},
		{value: "abc1-", want: true, why: "nothing to fold is nothing to fail"},
		{value: "Abc", want: false},
		{value: "", want: false},
	})

	run(t, "uppercase", verdict(validate.IsUppercase), []tc{
		{value: "ABC", want: true},
		{value: "ABC1-", want: true},
		{value: "Abc", want: false},
		{value: "", want: false},
	})

	run(t, "contains", validate.IsContains, []tc{
		{value: "abcdef", args: []string{"cd"}, want: true},
		{value: "abcdef", args: []string{"x"}, want: false},
		{value: "abcdef", args: []string{""}, want: true},
		{value: "abcdef", wantErr: true, why: "nothing to look for"},
	})

	run(t, "startswith", validate.IsStartswith, []tc{
		{value: "abcdef", args: []string{"ab"}, want: true},
		{value: "abcdef", args: []string{"bc"}, want: false},
		{value: "abcdef", wantErr: true},
	})

	run(t, "endswith", validate.IsEndswith, []tc{
		{value: "abcdef", args: []string{"ef"}, want: true},
		{value: "abcdef", args: []string{"de"}, want: false},
		{value: "abcdef", wantErr: true},
	})

	run(t, "country", validate.IsCountry, []tc{
		{value: "US", want: true},
		{value: "RU", want: true},
		{value: "us", want: true, why: "case is the client's choice"},
		{value: "rU", want: true},
		{value: "USA", want: false, why: "alpha-3 is rejected by the length check"},
		{value: "840", want: false, why: "numeric too"},
		{value: "", want: false},
		{value: "ZZ", want: true, why: "user-assigned code, accepted by x/text"},
		{value: "1A", wantErr: true, why: "two characters, but not a region"},
	})

	run(t, "currency", validate.IsCurrency, []tc{
		{value: "USD", want: true},
		{value: "EUR", want: true},
		{value: "XXX", want: true, why: `ISO 4217 "no currency"`},
		{value: "BTC", wantErr: true, why: "not an ISO currency"},
		{value: "", wantErr: true},
		{value: "usd", want: true, why: "folded, as in country"},
	})

	run(t, "locale", validate.IsLocale, []tc{
		{value: "en", want: true},
		{value: "en-US", want: true},
		{value: "zh-Hant-CN", want: true},
		{value: "en_US", want: true, why: "underscore accepted, though BCP 47 says hyphen"},
		{value: "xx", wantErr: true, why: "well-formed but not a language"},
		{value: "nonsense", wantErr: true},
		{value: "", want: false},
	})
}

// The ~string branch of the generic validators goes through reflect, and a
// declared type is the normal case in a request model.
func TestNamedStringTypes(t *testing.T) {
	type Email string
	type ID string
	type Link string

	if ok, err := validate.IsEmail(Email("a@b.com")); !ok || err != nil {
		t.Errorf("IsEmail(Email) = (%v, %v)", ok, err)
	}
	if ok, err := validate.IsEmail(Email("nope")); ok || err == nil {
		t.Errorf("IsEmail(Email) on garbage = (%v, %v)", ok, err)
	}
	if ok, err := validate.IsUuid(ID("0191b2a0-0000-7000-8000-000000000000")); !ok || err != nil {
		t.Errorf("IsUuid(ID) = (%v, %v)", ok, err)
	}
	if !validate.IsUrl(Link("http://a.com")) {
		t.Error("IsUrl(Link) = false")
	}
}

// url.URL is in the type set, and the branch that accepts it never looks at the
// value: a zero struct and a nil pointer are both reported valid.
func TestUrlStructsAreAlwaysValid(t *testing.T) {
	if !validate.IsUrl(url.URL{}) {
		t.Error("IsUrl(url.URL{}) = false, expected the documented always-true branch")
	}
	if !validate.IsUrl((*url.URL)(nil)) {
		t.Error("IsUrl((*url.URL)(nil)) = false, expected the documented always-true branch")
	}
}

func TestOneof(t *testing.T) {
	cases := []struct {
		value any
		args  []string
		want  bool
	}{
		{value: "b", args: []string{"a", "b"}, want: true},
		{value: "c", args: []string{"a", "b"}, want: false},
		{value: "a", want: false},
		{value: 2, args: []string{"1", "2"}, want: true},
		{value: int64(-3), args: []string{"-3"}, want: true},
		{value: uint8(7), args: []string{"7"}, want: true},
		{value: true, args: []string{"true"}, want: true},
		{value: []byte("x"), args: []string{"x"}, want: true},
		// -1 precision, so the trailing zero of 1.50 never reaches the compare.
		{value: 1.50, args: []string{"1.5"}, want: true},
		{value: 1.50, args: []string{"1.50"}, want: false},
		{value: float32(0.5), args: []string{"0.5"}, want: true},
		{value: stringer{}, args: []string{"stringer"}, want: true},
		{value: struct{ A int }{1}, args: []string{"{1}"}, want: true},
	}
	for _, c := range cases {
		if got := validate.IsOneof(c.value, c.args...); got != c.want {
			t.Errorf("IsOneof(%#v, %v) = %v, want %v", c.value, c.args, got, c.want)
		}
	}
}

type stringer struct{}

func (stringer) String() string { return "stringer" }
