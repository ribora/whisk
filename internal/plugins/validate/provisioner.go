package validate

import (
	"go/ast"

	"github.com/ribora/whisk/internal/shared"
)

// tag "passcode" -> func IsPasscode(…) in any file of the storage dir. A second
// result of error switches the emitter to the form that keeps it.
func isCustomValidator(idx shared.FuncIndex, tag string) (found, returnsErr bool) {
	fn := idx[customFuncName(tag)]
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

func customFuncName(tag string) string { return "Is" + shared.PascalCase(tag) }
