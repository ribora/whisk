package basic

import (
	"context"
	"testing"
)

// Whisk exists so that nothing reflects at run time, and nothing else here
// measures that.

type source struct {
	body   []byte
	scoped map[string][]string
}

func (s source) Lookup(scope, key string) string {
	if got := s.scoped[scope+"/"+key]; len(got) > 0 {
		return got[0]
	}
	return ""
}

func (s source) LookupAll(scope, key string) []string { return s.scoped[scope+"/"+key] }

func (s source) Body() []byte { return s.body }

func (s source) Context() context.Context { return context.Background() }

func BenchmarkParse(b *testing.B) {
	src := source{
		body: []byte(`{"email":"user@example.com","name":"Jane Doe"}`),
		scoped: map[string][]string{
			"path/orgId":           {"acme"},
			"header/Authorization": {"Bearer t0ken"},
			"query/tag":            {"eu", "beta"},
			"query/ttl":            {"14"},
		},
	}

	b.ReportAllocs()
	for b.Loop() {
		var inv Invite
		if err := inv.Parse(src); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidate(b *testing.B) {
	inv := Invite{
		OrgID: "acme",
		Token: "Bearer t0ken",
		Email: "user@example.com",
		Name:  "Jane Doe",
		Tags:  []string{"eu", "beta"},
		TTL:   14,
	}

	b.ReportAllocs()
	for b.Loop() {
		if err := inv.Validate(); err != nil {
			b.Fatal(err)
		}
	}
}

// Measured apart because only the failing path allocates.
func BenchmarkValidateFailing(b *testing.B) {
	inv := Invite{
		Email: "not-an-address",
		Name:  "..........................",
		Tags:  []string{"eu", "eu"},
		TTL:   99,
	}

	b.ReportAllocs()
	for b.Loop() {
		if err := inv.Validate(); err == nil {
			b.Fatal("expected the broken rules to report")
		}
	}
}
