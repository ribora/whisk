package scoped_body

import (
	"testing"

	"github.com/ribora/whisk/scan"
)

// The header and the query parameter are absent, so nothing overrides what
// the body would have written: a forged "userid" must not become the caller.
func TestBodyDoesNotReachScopedFields(t *testing.T) {
	req, err := scan.Into[Request](scan.FromBytes(
		[]byte(`{"userid":"forged","page":999,"name":"Ann","note":"kept"}`)))
	if err != nil {
		t.Fatal(err)
	}
	if req.UserID != "" {
		t.Errorf("UserID = %q, want it untouched by the body", req.UserID)
	}
	if req.Page != 0 {
		t.Errorf("Page = %d, want it untouched by the body", req.Page)
	}
	if req.Name != "Ann" {
		t.Errorf("Name = %q, want %q", req.Name, "Ann")
	}
	if req.Note != "kept" {
		t.Errorf("Note = %q, want %q: scan:\"-\" still comes from the body", req.Note, "kept")
	}
}
