package golang

import (
	"strings"

	"example.com/fyp/pkg/masking"
	gomasker "example.com/fyp/pkg/masking/golang"
	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/segmentation"
	gosegmenter "example.com/fyp/pkg/segmentation/golang"
	"example.com/fyp/pkg/validation"
	govalidator "example.com/fyp/pkg/validation/golang"
	sitter "github.com/smacker/go-tree-sitter"
)

// GoPlugin implements lang.LanguagePlugin for Go.
type GoPlugin struct {
	goParser           *GoParser
	symbolExtractor    *GoSymbolExtractor
	callGraphExtractor *GoCallGraphExtractor
	masker             *gomasker.GoMasker
	segmenter          *gosegmenter.GoSegmenter
	validator          *govalidator.GoValidator
}

// NewGoPlugin creates a new Go language plugin.
func NewGoPlugin() *GoPlugin {
	return &GoPlugin{
		goParser:           NewGoParser(),
		symbolExtractor:    NewGoSymbolExtractor(),
		callGraphExtractor: NewGoCallGraphExtractor(),
		masker:             gomasker.NewGoMasker(),
		segmenter:          gosegmenter.NewGoSegmenter(),
		validator:          govalidator.NewGoValidator(),
	}
}

// Parser returns the Go parser.
func (p *GoPlugin) Parser() parser.Parser {
	return p.goParser
}

// Parse parses content and returns the tree-sitter AST.
func (p *GoPlugin) Parse(content []byte) (*sitter.Tree, error) {
	return p.goParser.Parse(content)
}

// SymbolExtractor returns the Go symbol extractor.
func (p *GoPlugin) SymbolExtractor() parser.SymbolExtractor {
	return p.symbolExtractor
}

// CallGraphExtractor returns the Go call graph extractor.
func (p *GoPlugin) CallGraphExtractor() parser.CallGraphExtractor {
	return p.callGraphExtractor
}

// Masker returns the Go masker.
func (p *GoPlugin) Masker() masking.Masker {
	return p.masker
}

// Segmenter returns the Go segmenter.
func (p *GoPlugin) Segmenter() segmentation.Segmenter {
	return p.segmenter
}

// Validator returns the Go validator.
func (p *GoPlugin) Validator() validation.Validator {
	return p.validator
}

// Language returns the language name.
func (p *GoPlugin) Language() string {
	return "golang"
}

// Extensions returns the file extensions for Go.
func (p *GoPlugin) Extensions() []string {
	return []string{".go"}
}

// DetectLanguage detects if a file is Go based on its path.
func (p *GoPlugin) DetectLanguage(_ []byte, filePath string) bool {
	return strings.HasSuffix(filePath, ".go")
}
