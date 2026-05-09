package javascript

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

// JavaScriptPlugin implements lang.LanguagePlugin for JavaScript.
type JavaScriptPlugin struct {
	parser             *ecmascript.ECMAScriptParser
	symbolExtractor    *ecmascript.ECMAScriptSymbolExtractor
	callGraphExtractor *ecmascript.ECMAScriptCallGraphExtractor
	masker             *ecmamasker.ECMAScriptMasker
	segmenter          *ecmasegmenter.ECMAScriptSegmenter
	validator          *ecmavalidator.ECMAScriptValidator
}

// NewJavaScriptPlugin creates a JavaScript plugin.
func NewJavaScriptPlugin() *JavaScriptPlugin {
	return &JavaScriptPlugin{
		parser:             ecmascript.NewJavaScriptParser(),
		symbolExtractor:    ecmascript.NewECMAScriptSymbolExtractor(),
		callGraphExtractor: ecmascript.NewECMAScriptCallGraphExtractor(),
		masker:             ecmamasker.NewJavaScriptMasker(),
		segmenter:          ecmasegmenter.NewJavaScriptSegmenter(),
		validator:          ecmavalidator.NewJavaScriptValidator(),
	}
}

// Parser returns the JavaScript parser.
func (p *JavaScriptPlugin) Parser() parser.Parser {
	return p.parser
}

// Parse parses source content.
func (p *JavaScriptPlugin) Parse(content []byte) (*sitter.Tree, error) {
	return p.parser.Parse(content)
}

// SymbolExtractor returns symbol extraction implementation.
func (p *JavaScriptPlugin) SymbolExtractor() parser.SymbolExtractor {
	return p.symbolExtractor
}

// CallGraphExtractor returns heuristic callgraph extractor (disabled).
func (p *JavaScriptPlugin) CallGraphExtractor() parser.CallGraphExtractor {
	return p.callGraphExtractor
}

// Masker returns the JavaScript masker.
func (p *JavaScriptPlugin) Masker() masking.Masker {
	return p.masker
}

// Segmenter returns the JavaScript segmenter.
func (p *JavaScriptPlugin) Segmenter() segmentation.Segmenter {
	return p.segmenter
}

// Validator returns the JavaScript validator.
func (p *JavaScriptPlugin) Validator() validation.Validator {
	return p.validator
}

// Language returns plugin language name.
func (p *JavaScriptPlugin) Language() string {
	return "javascript"
}

// Extensions returns supported file extensions.
func (p *JavaScriptPlugin) Extensions() []string {
	return []string{".js", ".jsx", ".mjs", ".cjs"}
}

// DetectLanguage checks if path looks like JavaScript.
func (p *JavaScriptPlugin) DetectLanguage(content []byte, filePath string) bool {
	normalized := strings.ToLower(filePath)
	for _, ext := range p.Extensions() {
		if strings.HasSuffix(normalized, ext) {
			return true
		}
	}
	return false
}
