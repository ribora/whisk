package validate_test

import (
	"net/mail"
	"testing"
	"uuid"

	guid "github.com/google/uuid"
	"gitlab.com/gorib/whisk/validate"
)

// The type sets of the generic validators are wide because a request model can
// declare the field as the parsed type rather than a string. Each branch of the
// type switch is a separate path to the same verdict.
func TestEmailValueShapes(t *testing.T) {
	addr := mail.Address{Name: "Peter", Address: "p@example.com"}

	if ok, err := validate.IsEmail(addr); !ok || err != nil {
		t.Errorf("IsEmail(mail.Address) = (%v, %v)", ok, err)
	}
	if ok, err := validate.IsEmail(&addr); !ok || err != nil {
		t.Errorf("IsEmail(*mail.Address) = (%v, %v)", ok, err)
	}
	// A parsed address is trusted as-is: the struct branch never re-parses, so
	// even an empty one passes.
	if ok, err := validate.IsEmail(mail.Address{}); !ok || err != nil {
		t.Errorf("IsEmail(zero mail.Address) = (%v, %v)", ok, err)
	}
}

func TestUuidValueShapes(t *testing.T) {
	const text = "0191b2a0-0000-7000-8000-000000000000"
	g := guid.MustParse(text)
	std, err := uuid.Parse(text)
	if err != nil {
		t.Fatalf("uuid.Parse: %v", err)
	}

	if ok, err := validate.IsUuid(g); !ok || err != nil {
		t.Errorf("IsUuid(guid.UUID) = (%v, %v)", ok, err)
	}
	if ok, err := validate.IsUuid(&g); !ok || err != nil {
		t.Errorf("IsUuid(*guid.UUID) = (%v, %v)", ok, err)
	}
	if ok, err := validate.IsUuid(std); !ok || err != nil {
		t.Errorf("IsUuid(uuid.UUID) = (%v, %v)", ok, err)
	}
	if ok, err := validate.IsUuid(&std); !ok || err != nil {
		t.Errorf("IsUuid(*uuid.UUID) = (%v, %v)", ok, err)
	}

	// The version argument reads the parsed value, whichever shape carried it.
	if ok, err := validate.IsUuid(g, "4"); ok || err == nil {
		t.Errorf("IsUuid(guid.UUID, 4) on a v7 = (%v, %v), want a mismatch", ok, err)
	}
}

// IsOneof compares the rendered value, so every numeric width has to render the
// same way strconv would — a missing case falls through to fmt.Sprint and would
// still often match, which is why each one is named here.
func TestOneofNumericWidths(t *testing.T) {
	cases := []struct {
		label string
		value any
		arg   string
	}{
		{"int8", int8(-8), "-8"},
		{"int16", int16(-16), "-16"},
		{"int32", int32(-32), "-32"},
		{"int64", int64(-64), "-64"},
		{"int", int(1), "1"},
		{"uint", uint(2), "2"},
		{"uint8", uint8(3), "3"},
		{"uint16", uint16(4), "4"},
		{"uint32", uint32(5), "5"},
		{"uint64", uint64(6), "6"},
		{"float32", float32(1.25), "1.25"},
		{"float64", float64(2.5), "2.5"},
	}
	for _, c := range cases {
		if !validate.IsOneof(c.value, c.arg) {
			t.Errorf("IsOneof(%s %v, %q) = false", c.label, c.value, c.arg)
		}
		if validate.IsOneof(c.value, "other") {
			t.Errorf("IsOneof(%s %v, \"other\") = true", c.label, c.value)
		}
	}
}
