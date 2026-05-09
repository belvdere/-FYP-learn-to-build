package python

import (
	"strings"

	"example.com/fyp/pkg/masking"
	pythonmasker "example.com/fyp/pkg/masking/python"
	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/segmentation"
	pythonsegmenter "example.com/fyp/pkg/segmentation/python"
	"example.com/fyp/pkg/validation"
	pythonvalidator "example.com/fyp/pkg/validation/python"
	sitter "github.com/smacker/go-tree-sitter"
)

// PythonPlugin implements lang.LanguagePlugin for Python.
type PythonPlugin struct {
	pythonParser       *PythonParser
	symbolExtractor    *PythonSymbolExtractor
	callGraphExtractor *PythonCallGraphExtractor
	masker             *pythonmasker.PythonMasker
	segmenter          *pythonsegmenter.PythonSegmenter
	validator          *pythonvalidator.PythonValidator
}

// NewPythonPlugin creates a new Python language plugin.
func NewPythonPlugin() *PythonPlugin {
	return &PythonPlugin{
		pythonParser:       NewPythonParser(),
		symbolExtractor:    NewPythonSymbolExtractor(),
		callGraphExtractor: NewPythonCallGraphExtractor(),
		masker:             pythonmasker.NewPythonMasker(),
		segmenter:          pythonsegmenter.NewPythonSegmenter(),
		validator:          pythonvalidator.NewPythonValidator(),
	}
}

// Parser returns the Python parser.
func (p *PythonPlugin) Parser() parser.Parser {
	return p.pythonParser
}

// Parse parses content and returns the tree-sitter AST.
func (p *PythonPlugin) Parse(content []byte) (*sitter.Tree, error) {
	return p.pythonParser.Parse(content)
}

// SymbolExtractor returns the Python symbol extractor.
func (p *PythonPlugin) SymbolExtractor() parser.SymbolExtractor {
	return p.symbolExtractor
}

// CallGraphExtractor returns the Python call graph extractor.
func (p *PythonPlugin) CallGraphExtractor() parser.CallGraphExtractor {
	return p.callGraphExtractor
}

// Masker returns the Python masker.
func (p *PythonPlugin) Masker() masking.Masker {
	return p.masker
}

// Segmenter returns the Python segmenter.
func (p *PythonPlugin) Segmenter() segmentation.Segmenter {
	return p.segmenter
}

// Validator returns the Python validator.
func (p *PythonPlugin) Validator() validation.Validator {
	return p.validator
}

// Language returns the language name.
func (p *PythonPlugin) Language() string {
	return "python"
}

// Extensions returns file extensions for Python.
func (p *PythonPlugin) Extensions() []string {
	return []string{".py"}
}

// DetectLanguage detects if a file is Python based on its extension.
func (p *PythonPlugin) DetectLanguage(content []byte, filePath string) bool {
	return strings.HasSuffix(strings.ToLower(filePath), ".py")
}
