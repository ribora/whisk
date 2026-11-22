package validate

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ribora/whisk/internal/shared"
)

// ---- custom (built-in + user-defined Is<Tag>) ----

type customHandler struct {
	resolve func(tag string) tagResolution
}

func (h customHandler) handle(buf *bytes.Buffer, f field, t Tag, ctx context) (bool, error) {
	res := h.resolve(t.Name)
	if !res.Supported {
		return false, nil
	}
	// `datetime` on time.Time is implicit — it's always a valid time value.
	if t.Name == "datetime" && f.Type != "string" && f.Type != "*string" {
		if f.Type == "time.Time" || (f.IsPointer && f.Type == "*time.Time") {
			fmt.Fprintf(buf, "\t// %s is time.Time, datetime validation is implicit\n", f.effectiveName(ctx.Prefix))
		}
		return true, nil
	}

	pascalTag := shared.PascalCase(t.Name)
	eff := f.asBasic(f.effectiveName(ctx.Prefix))

	args := ""
	if t.HasValue {
		for _, part := range strings.Fields(t.Value) {
			args += fmt.Sprintf(", %q", part)
		}
	}

	var checkExpr, errCap string
	if res.ReturnsError {
		checkExpr = fmt.Sprintf("ok, errCapture := %s.Is%s(%s%s); !ok", res.Prefix, pascalTag, eff, args)
		errCap = "errCapture"
	} else {
		checkExpr = fmt.Sprintf("!%s.Is%s(%s%s)", res.Prefix, pascalTag, eff, args)
	}

	// Same kind buckets as required.
	emitValidationError(buf, checkExpr, fieldSegmentCode(f, ctx.ExtractTags), t.Name, t.Value, requiredKind(f), errCap)
	return true, nil
}
