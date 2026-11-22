package embed_path

import (
	"errors"
	"testing"

	"github.com/ribora/whisk/validate"
)

// The embed is read flat, so the client sent "name", not "role.name".
func TestEmbedAddsNoPathSegment(t *testing.T) {
	err := (&UpdateRoleRequest{ID: 1}).Validate()
	var ve *validate.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("Validate() = %v, want a *validate.ValidationError", err)
	}
	if got := ve.Segments.Format(validate.JSONFormatter); got != "name" {
		t.Errorf("path = %q, want %q", got, "name")
	}
}
