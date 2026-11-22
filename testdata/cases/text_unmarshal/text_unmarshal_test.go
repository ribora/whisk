package text_unmarshal

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/ribora/whisk/scan"
)

// values is a Source backed by a map, so a case can be exercised without any
// transport. The key is "scope/name" because scopes are opaque strings.
type values map[string][]string

func (v values) Lookup(scope, key string) string {
	if got := v[scope+"/"+key]; len(got) > 0 {
		return got[0]
	}
	return ""
}

func (v values) LookupAll(scope, key string) []string { return v[scope+"/"+key] }

func (v values) Body() []byte { return nil }

func (v values) Context() context.Context { return context.Background() }

const (
	idA = "0f8fad5b-d9cb-469f-a165-70867728950e"
	idB = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
)

func TestDecodesThroughUnmarshalText(t *testing.T) {
	req, err := scan.Into[Request](values{
		"path/id":        {idA},
		"query/parent":   {idB},
		"query/related":  {idA + "," + idB},
		"query/assignee": {idA, idB},
		"query/contact":  {"user@example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := req.ID.String(); got != idA {
		t.Errorf("ID = %s, want %s", got, idA)
	}
	if req.Parent == nil {
		t.Fatal("Parent was not allocated")
	}
	if got := req.Parent.String(); got != idB {
		t.Errorf("Parent = %s, want %s", got, idB)
	}
	if len(req.Related) != 2 {
		t.Errorf("Related has %d elements, want 2", len(req.Related))
	}
	if len(req.Assignee) != 2 {
		t.Errorf("Assignee has %d elements, want 2", len(req.Assignee))
	}
	if req.Contact != "user@example.com" {
		t.Errorf("Contact = %q", req.Contact)
	}
}

// An absent parameter must leave a pointer field nil rather than allocate it.
func TestAbsentPointerStaysNil(t *testing.T) {
	req, err := scan.Into[Request](values{"path/id": {idA}})
	if err != nil {
		t.Fatal(err)
	}
	if req.Parent != nil {
		t.Errorf("Parent = %v, want nil", req.Parent)
	}
	if req.Related != nil {
		t.Errorf("Related = %v, want nil", req.Related)
	}
}

func TestDecoderErrorIsPayloadError(t *testing.T) {
	_, err := scan.Into[Request](values{"path/id": {"not-a-uuid"}})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, scan.ErrPayload) {
		t.Errorf("error %v does not wrap scan.ErrPayload", err)
	}
}

// The slice-of-pointer field is left to the hook, so it must stay empty rather
// than dereference a nil element.
func TestPointerSliceIsNotBound(t *testing.T) {
	req, err := scan.Into[Request](values{"path/id": {idA}, "query/watcher": {idA, idB}})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Watchers) != 0 {
		t.Errorf("Watchers has %d elements, want 0", len(req.Watchers))
	}
	var _ []*uuid.UUID = req.Watchers
}
