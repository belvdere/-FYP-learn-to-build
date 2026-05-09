package typescript

import (
	"strings"

	"example.com/fyp/pkg/masking"
	ecmamasker "example.com/fyp/pkg/masking/ecmascript"
	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/parser/ecmascript"
	"example.com/fyp/pkg/segmentation"
	ecmasegmenter "example.com/fyp/pkg/segmentation/ecmascript"
	"example.com/fyp/pkg/validation"
	ecmavalidator "example.com/fyp/pkg/validation/ecmascript"
	sitter "github.com/smacker/go-tree-sitter"
)

// TypeScriptPlugin implements lang.LanguagePlugin for TypeScript.
type TypeScriptPlugin struct {
	parser             *ecmascript.ECMAScriptParser
	symbolExtractor    *ecmascript.ECMAScriptSymbolExtractor
	callGraphExtractor *ecmascript.ECMAScriptCallGraphExtractor
	masker             *ecmamasker.ECMAScriptMasker
	segmenter          *ecmasegmenter.ECMAScriptSegmenter
	validator          *ecmavalidator.ECMAScriptValidator
}

// NewTypeScriptPlugin creates a TypeScript plugin.
func NewTypeScriptPlugin() *TypeScriptPlugin {
	return &TypeScriptPlugin{
		parser:             ecmascript.NewTypeScriptParser(),
		symbolExtractor:    ecmascript.NewECMAScriptSymbolExtractor(),
		callGraphExtractor: ecmascript.NewECMAScriptCallGraphExtractor(),
		masker:             ecmamasker.NewTypeScriptMasker(),
		segmenter:          ecmasegmenter.NewTypeScriptSegmenter(),
		validator:          ecmavalidator.NewTypeScriptValidator(),
	}
}

// Parser returns the TypeScript parser.
func (p *TypeScriptPlugin) Parser() parser.Parser {
	return p.parser
}

// Parse parses source content.
func (p *TypeScriptPlugin) Parse(content []byte) (*sitter.Tree, error) {
	return p.parser.Parse(content)
}

// SymbolExtractor returns symbol extraction implementation.
func (p *TypeScriptPlugin) SymbolExtractor() parser.SymbolExtractor {
	return p.symbolExtractor
}

// CallGraphExtractor returns heuristic callgraph extractor (disabled).
func (p *TypeScriptPlugin) CallGraphExtractor() parser.CallGraphExtractor {
	return p.callGraphExtractor
}

// Masker returns the TypeScript masker.
func (p *TypeScriptPlugin) Masker() masking.Masker {
	return p.masker
}

// Segmenter returns the TypeScript segmenter.
func (p *TypeScriptPlugin) Segmenter() segmentation.Segmenter {
	return p.segmenter
}

// Validator returns the TypeScript validator.
func (p *TypeScriptPlugin) Validator() validation.Validator {
	return p.validator
}

// Language returns plugin language name.
func (p *TypeScriptPlugin) Language() string {
	return "typescript"
}

// Extensions returns supported TypeScript file extensions.
func (p *TypeScriptPlugin) Extensions() []string {
	return []string{".ts", ".tsx", ".mts", ".cts"}
}

// DetectLanguage checks if path looks like TypeScript.
func (p *TypeScriptPlugin) DetectLanguage(content []byte, filePath string) bool {
	normalized := strings.ToLower(filePath)
	for _, ext := range p.Extensions() {
		if strings.HasSuffix(normalized, ext) {
			return true
		}
	}
	return false
}
