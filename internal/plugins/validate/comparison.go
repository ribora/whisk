package validate

import (
	"bytes"
	"fmt"
)

// comparisonOps maps a comparison tag name to the inverted operator used
// in the generated check ("if FAIL"). E.g. `min=8` → fail when `len(x) < 8`.
var comparisonOps = map[string]string{
	"min": "<", "gte": "<", "max": ">", "lte": ">",
	"gt": "<=", "lt": ">=", "eq": "!=", "equal": "!=",
	"ne": "==", "neq": "==", "len": "!=",
}

// ---- comparison (min/max/gt/lt/eq/ne/len) ----

type comparisonHandler struct{}

func (comparisonHandler) handle(buf *bytes.Buffer, f field, t Tag, ctx context) (bool, error) {
	cmp, ok := comparisonOps[t.Name]
	if !ok {
		return false, nil
	}

	eff := f.effectiveName(ctx.Prefix)
	var (
		checkExpr string
		kind      string
	)

	isTimeField := f.shape() == shapeTime
	isTimeString := f.shape() == shapeString && isTimeValue(t.Value)

	switch {
	case isTimeField || isTimeString:
		kind = kindTime
		switch t.Name {
		case "min", "gte":
			checkExpr = fmt.Sprintf("!validate.IsTimeMin(%s, %q)", eff, t.Value)
		case "max", "lte":
			checkExpr = fmt.Sprintf("!validate.IsTimeMax(%s, %q)", eff, t.Value)
		case "gt":
			checkExpr = fmt.Sprintf("!validate.IsTimeGT(%s, %q)", eff, t.Value)
		case "lt":
			checkExpr = fmt.Sprintf("!validate.IsTimeLT(%s, %q)", eff, t.Value)
		default:
			// eq/ne/len against an instant: "this exact moment and no other" is not
			// something a request field asks for, and the code for it did not even
			// compile. A string holding a date can still be matched literally.
			if !isTimeString {
				return false, fmt.Errorf(
					"%s doesn't make sense for time field %q, an instant is bounded with gte/lte rather than matched",
					t.Name, f.Name)
			}
			checkExpr = fmt.Sprintf("%s %s %q", eff, cmp, t.Value)
			kind = kindValue
		}

	case f.shape() == shapeString:
		if t.Name == "eq" || t.Name == "equal" || t.Name == "ne" || t.Name == "neq" {
			checkExpr = fmt.Sprintf("%s %s %q", eff, cmp, t.Value)
			kind = kindValue
		} else {
			checkExpr = fmt.Sprintf("utf8.RuneCountInString(%s) %s %s", f.asBasic(eff), cmp, t.Value)
			kind = kindStr
		}

	case f.shape() == shapeLen:
		checkExpr = fmt.Sprintf("len(%s) %s %s", eff, cmp, t.Value)
		kind = kindSlice

	case f.shape() == shapeBool:
		if (t.Value != "true" && t.Value != "false") || (cmp != "==" && cmp != "!=") {
			return false, fmt.Errorf("wrong comparison for field %s", f.Name)
		}
		expr := eff
		if (cmp == "==" && t.Value == "false") || (cmp == "!=" && t.Value == "true") {
			expr = "!" + expr
		}
		checkExpr = expr
		kind = kindValue

	default:
		checkExpr = fmt.Sprintf("%s %s %s", eff, cmp, t.Value)
		kind = kindValue
	}

	emitValidationError(buf, checkExpr, fieldSegmentCode(f, ctx.ExtractTags), t.Name, t.Value, kind, "")
	return true, nil
}
