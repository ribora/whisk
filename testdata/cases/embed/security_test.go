package embed

import (
	"context"
	"testing"

	"gitlab.com/gorib/whisk/scan"
)

// fakeSource is a scan.Source whose body, header, query and path scopes can
// be set independently — letting a test pit a request body against values
// an embed sources from a non-body scope.
type fakeSource struct {
	body   []byte
	values map[[2]string]string
}

var _ scan.Source = (*fakeSource)(nil)

func (f *fakeSource) Lookup(scope, key string) string   { return f.values[[2]string{scope, key}] }
func (f *fakeSource) LookupAll(string, string) []string { return nil }
func (f *fakeSource) Body() []byte                      { return f.body }
func (f *fakeSource) Context() context.Context          { return context.Background() }

// TestParse_bodyIsIgnoredForTagOnlyStruct is the IDOR regression. Every
// field on ListRequest carries a scan tag (header / query / path / embed),
// so the generator emits Parse WITHOUT a body unmarshal — there is no
// bucket-1 field for json.Unmarshal to write into. A malicious body that
// tries to forge values the request reads from non-body scopes is dropped
// on the floor.
//
// If render.go ever regresses and starts emitting an unconditional body
// unmarshal, encoding/json would land the forged values on s.Auth.Token /
// s.Pagination.* (case-insensitive field-name match) before the delegates
// ran. The query/header values still win in that hypothetical because
// delegates run AFTER body, but the assertions below fail-closed if the
// forged body ever leaks into the struct at all.
func TestParse_bodyIsIgnoredForTagOnlyStruct(t *testing.T) {
	src := &fakeSource{
		// Forged: client tries to set the header-sourced token and the
		// query-sourced pagination via the JSON body. With no bucket-1
		// field, this body is never unmarshalled.
		body: []byte(`{"auth":{"token":"forged-by-client"},` +
			`"pagination":{"page":999,"per_page":50}}`),
		values: map[[2]string]string{
			{"header", "Authorization"}: "token-from-context",
			{"query", "page"}:           "2",
			{"query", "per_page"}:       "20",
			{"path", "userId"}:          "u-42",
		},
	}

	req := &ListRequest{}
	if err := req.Parse(src); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if req.Auth.Token != "token-from-context" {
		t.Errorf("Auth.Token = %q, want %q (forged body must not leak through)",
			req.Auth.Token, "token-from-context")
	}
	if req.Pagination.Page != 2 {
		t.Errorf("Pagination.Page = %d, want 2 (forged %d)", req.Pagination.Page, 999)
	}
	if req.Pagination.PerPage != 20 {
		t.Errorf("Pagination.PerPage = %d, want 20 (forged %d)", req.Pagination.PerPage, 50)
	}
	if req.UserID != "u-42" {
		t.Errorf("UserID = %q, want %q", req.UserID, "u-42")
	}
}
