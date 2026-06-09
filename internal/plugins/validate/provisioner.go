package validate

import (
	"go/ast"

	"gitlab.com/gorib/whisk/internal/shared"
)

// isCustomValidator reports whether storagePath holds `<tag>.go` declaring
// `func Is<PascalCase(tag)>(...)`. returnsErr is true iff that function's
// second result is `error` — then the generator emits the two-result form
// and threads the captured error into ValidationError.Err.
func isCustomValidator(tag, storagePath string) (found, returnsErr bool) {
	fn := shared.FindFunc(storagePath, tag+".go", "Is"+shared.PascalCase(tag))
	if fn == nil {
		return false, false
	}
	// We don't enforce the first result being bool — the compiler catches it.
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 1 {
		if id, ok := fn.Type.Results.List[1].Type.(*ast.Ident); ok && id.Name == "error" {
			return true, true
		}
	}
	return true, false
}
