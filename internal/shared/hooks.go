package shared

import (
	"fmt"
	"go/types"
)

// ownHookSignature reports whether *T defines `name(...) error` directly.
// paramType is the expected single parameter as types.Type.String() (e.g.
// "gitlab.com/gorib/whisk/scan.Source"), or "" for a parameterless hook.
//
// Methods promoted from an embed (len(Index) > 1) are skipped: the embed's
// own generated code already calls its hook, so picking it up here would
// double-call it.
func ownHookSignature(named *types.Named, name, paramType string) (bool, error) {
	ms := types.NewMethodSet(types.NewPointer(named))
	for i := 0; i < ms.Len(); i++ {
		sel := ms.At(i)
		if sel.Obj().Name() != name {
			continue
		}
		if len(sel.Index()) != 1 {
			continue
		}
		sig, ok := sel.Obj().Type().(*types.Signature)
		if !ok {
			continue
		}
		if err := checkHookSig(sig, named.Obj().Name(), name, paramType); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func checkHookSig(sig *types.Signature, structName, hookName, paramType string) error {
	wantParams := 0
	if paramType != "" {
		wantParams = 1
	}
	if sig.Params().Len() != wantParams || sig.Results().Len() != 1 {
		return fmt.Errorf("%s.%s must have signature (%s) error", structName, hookName, paramType)
	}
	if paramType != "" && sig.Params().At(0).Type().String() != paramType {
		return fmt.Errorf("%s.%s must take %s", structName, hookName, paramType)
	}
	if sig.Results().At(0).Type().String() != "error" {
		return fmt.Errorf("%s.%s must return error", structName, hookName)
	}
	return nil
}

// HasOwnValidateMethod reports whether *named defines `Validate() error`.
// Promoted-from-embed methods count: a value embedding such a type really
// does carry a usable Validate at the final compile.
func HasOwnValidateMethod(named *types.Named) bool {
	return hasMethodReturningError(named, "Validate", 0)
}

// HasOwnValidateHook reports whether *named defines its own `validate() error`
// (not promoted from an embed).
func HasOwnValidateHook(named *types.Named) bool {
	ok, _ := ownHookSignature(named, "validate", "")
	return ok
}

func hasMethodReturningError(named *types.Named, name string, params int) bool {
	ms := types.NewMethodSet(types.NewPointer(named))
	for i := 0; i < ms.Len(); i++ {
		sel := ms.At(i)
		if sel.Obj().Name() != name {
			continue
		}
		sig, ok := sel.Obj().Type().(*types.Signature)
		if !ok {
			continue
		}
		if sig.Params().Len() == params && sig.Results().Len() == 1 &&
			sig.Results().At(0).Type().String() == "error" {
			return true
		}
	}
	return false
}
