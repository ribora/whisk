package embed_body

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitlab.com/gorib/whisk/scan"
)

type request struct {
	body    string
	headers map[string]string
}

func (r request) Lookup(scope, key string) string {
	if scope != "header" {
		return ""
	}
	return r.headers[key]
}

func (r request) LookupAll(string, string) []string { return nil }
func (r request) Body() []byte                      { return []byte(r.body) }
func (r request) Context() context.Context          { return context.Background() }

// "actor" and "note" match the embed fields' names case-insensitively; the
// parent's decode must not reach them, while Note's own Parse reads "note" flat.
func TestEmbedFieldsStayOutOfParentBody(t *testing.T) {
	req := &UpdateUser{Limit: 10}
	err := req.Parse(request{
		body:    `{"actor":"forged","note":"hi","reason":"why","name":"Ann","until":"2026-01-02T00:00:00Z"}`,
		headers: map[string]string{"X-User-Id": "u-1"},
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if req.Actor.ID != "u-1" {
		t.Errorf("Actor.ID = %q, want %q", req.Actor.ID, "u-1")
	}
	if req.Note.Text != "hi" {
		t.Errorf("Note.Text = %q, want %q", req.Note.Text, "hi")
	}
	if req.Reason != "why" {
		t.Errorf("Reason = %q, want %q", req.Reason, "why")
	}
	if req.Name != "Ann" {
		t.Errorf("Name = %q, want %q", req.Name, "Ann")
	}
	if want := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC); !req.Until.Equal(want) {
		t.Errorf("Until = %v, want %v", req.Until, want)
	}
	if req.Limit != 10 {
		t.Errorf("Limit = %d, want the preset 10 kept when the body omits it", req.Limit)
	}
}

func TestBodyShapeErrorIsPayloadError(t *testing.T) {
	err := (&UpdateUser{}).Parse(request{body: `{"name":1}`})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, scan.ErrPayload) {
		t.Errorf("error %v does not wrap scan.ErrPayload", err)
	}
}
