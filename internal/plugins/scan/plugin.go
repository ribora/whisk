package scan

import (
	"github.com/ribora/whisk/internal/generator"
	"github.com/ribora/whisk/internal/shared"
)

type Plugin struct {
	Format Format
}

func New(format Format) *Plugin {
	if format.Tag == "" {
		format = FormatJSON
	}
	return &Plugin{Format: format}
}

func (p *Plugin) Name() string { return "scan" }

func (p *Plugin) Generate(structs []shared.StructInfo) (generator.Section, error) {
	lexems := make([]lexem, 0, len(structs))
	var warnings []string
	for _, s := range structs {
		lex, warns, err := walk(s, p.Format.Tag)
		if err != nil {
			return generator.Section{}, err
		}
		warnings = append(warnings, warns...)
		lexems = append(lexems, lex)
	}
	if len(lexems) == 0 {
		return generator.Section{}, nil
	}
	code := render(lexems, p.Format)
	return generator.Section{Code: code, Imports: neededImports(lexems, p.Format), Warnings: warnings}, nil
}
