package validate

import (
	"go/ast"

	"gitlab.com/gorib/whisk/internal/shared"
)

// tag "passcode" -> storagePath/passcode.go, func IsPasscode(…). A second
// result of error switches the emitter to the form that keeps it.
func isCustomValidator(tag, storagePath string) (found, returnsErr bool) {
	fn := shared.FindFunc(storagePath, tag+".go", "Is"+shared.PascalCase(tag))
	if fn == nil {
		return false, false
	}
	// The first result is left to the compiler.
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 1 {
		if id, ok := fn.Type.Results.List[1].Type.(*ast.Ident); ok && id.Name == "error" {
			return true, true
		}
	}
	return true, false
}
