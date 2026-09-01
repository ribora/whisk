package generator_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/gorib/whisk/internal/generator"
	"gitlab.com/gorib/whisk/internal/plugins/scan"
	"gitlab.com/gorib/whisk/internal/plugins/validate"
	"gitlab.com/gorib/whisk/internal/shared"
	"gitlab.com/gorib/whisk/internal/testmodule"
)

// Loading is the Go type-checker's time, not whisk's, so it happens once and the
// benchmark measures only what the plugins do.
func BenchmarkRender(b *testing.B) {
	structs, pkgName := loadModel(b, 40)
	plugins := []generator.Plugin{
		scan.New(scan.FormatJSON),
		validate.New(validate.Options{ExtractTags: []string{"json"}}),
	}

	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := generator.Render(pkgName, structs, plugins); err != nil {
			b.Fatal(err)
		}
	}
}

func loadModel(b *testing.B, n int) ([]shared.StructInfo, string) {
	b.Helper()

	var src strings.Builder
	src.WriteString("package sample\n\nimport \"time\"\n\n")
	src.WriteString("type Item struct {\n\tName string `validate:\"required,min=1\" json:\"name\"`\n}\n\n")
	for i := range n {
		fmt.Fprintf(&src, `type Model%d struct {
	ID       string            `+"`scan:\"path=id\" validate:\"required,uuid\" json:\"id\"`"+`
	Page     int               `+"`scan:\"query=page\" validate:\"gte=1,lte=100\" json:\"page\"`"+`
	Tags     []string          `+"`scan:\"query=tag,repeated\" validate:\"dive,min=2\" json:\"tags\"`"+`
	Email    string            `+"`validate:\"required,email\" json:\"email\"`"+`
	When     time.Time         `+"`validate:\"gt=2020-01-01\" json:\"when\"`"+`
	Items    []Item            `+"`validate:\"dive\" json:\"items\"`"+`
	Labels   map[string]string `+"`validate:\"dive,min=2\" json:\"labels\"`"+`
	Optional *string           `+"`json:\"optional\"`"+`
}

`, i)
	}

	dir := testmodule.New(&testing.T{}, src.String())
	pkgName, structs, err := shared.Load(dir, filepath.Join(dir, "sample.go"))
	if err != nil {
		b.Fatal(err)
	}
	if len(structs) != n+1 {
		b.Fatalf("loaded %d structs, want %d", len(structs), n+1)
	}
	return structs, pkgName
}

// Loading is what a real run actually waits on. Measured next to Render so the
// ratio is on the record rather than assumed.
func BenchmarkLoad(b *testing.B) {
	var src strings.Builder
	src.WriteString("package sample\n\nimport \"time\"\n\n")
	src.WriteString("type Item struct {\n\tName string `validate:\"required\" json:\"name\"`\n}\n\n")
	for i := range 40 {
		fmt.Fprintf(&src, "type Model%d struct {\n\tID string `scan:\"path=id\" validate:\"required\" json:\"id\"`\n\tWhen time.Time `json:\"when\"`\n}\n\n", i)
	}
	dir := testmodule.New(&testing.T{}, src.String())
	target := filepath.Join(dir, "sample.go")

	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := shared.Load(dir, target); err != nil {
			b.Fatal(err)
		}
	}
}
