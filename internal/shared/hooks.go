package shared

import (
	"fmt"
	"go/types"
)

// paramType is the single parameter as types.Type.String(), or "" for none.
// A method promoted from an embed is skipped: the embed's own generated code
// calls that hook already.
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

// A promoted method counts here: a value embedding such a type really does
// carry a usable Validate at the final compile.
func HasOwnValidateMethod(named *types.Named) bool {
	return hasMethodReturningError(named, "Validate", 0)
}

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
