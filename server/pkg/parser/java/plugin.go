package java

import (
	"strings"

	"example.com/fyp/pkg/masking"
	javamasker "example.com/fyp/pkg/masking/java"
	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/segmentation"
	javasegmenter "example.com/fyp/pkg/segmentation/java"
	"example.com/fyp/pkg/validation"
	javavalidator "example.com/fyp/pkg/validation/java"
	sitter "github.com/smacker/go-tree-sitter"
)

// JavaPlugin implements lang.LanguagePlugin for Java.
type JavaPlugin struct {
	javaParser         *JavaParser
	symbolExtractor    *JavaSymbolExtractor
	callGraphExtractor *JavaCallGraphExtractor
	masker             *javamasker.JavaMasker
	segmenter          *javasegmenter.JavaSegmenter
	validator          *javavalidator.JavaValidator
}

// NewJavaPlugin creates a new Java language plugin.
func NewJavaPlugin() *JavaPlugin {
	return &JavaPlugin{
		javaParser:         NewJavaParser(),
		symbolExtractor:    NewJavaSymbolExtractor(),
		callGraphExtractor: NewJavaCallGraphExtractor(),
		masker:             javamasker.NewJavaMasker(),
		segmenter:          javasegmenter.NewJavaSegmenter(),
		validator:          javavalidator.NewJavaValidator(),
	}
}

// Parser returns the Java parser.
func (p *JavaPlugin) Parser() parser.Parser {
	return p.javaParser
}

// Parse parses content and returns the tree-sitter AST.
func (p *JavaPlugin) Parse(content []byte) (*sitter.Tree, error) {
	return p.javaParser.Parse(content)
}

// SymbolExtractor returns the Java symbol extractor.
func (p *JavaPlugin) SymbolExtractor() parser.SymbolExtractor {
	return p.symbolExtractor
}

// CallGraphExtractor returns the Java call graph extractor.
func (p *JavaPlugin) CallGraphExtractor() parser.CallGraphExtractor {
	return p.callGraphExtractor
}

// Masker returns the Java masker for masking engine.
func (p *JavaPlugin) Masker() masking.Masker {
	return p.masker
}

// Segmenter returns the Java segmenter for code segmentation.
func (p *JavaPlugin) Segmenter() segmentation.Segmenter {
	return p.segmenter
}

// Validator returns the Java validator.
func (p *JavaPlugin) Validator() validation.Validator {
	return p.validator
}

// Language returns the language name.
func (p *JavaPlugin) Language() string {
	return "java"
}

// Extensions returns the file extensions for Java.
func (p *JavaPlugin) Extensions() []string {
	return []string{".java"}
}

// DetectLanguage detects if a file is Java based on its path.
func (p *JavaPlugin) DetectLanguage(content []byte, filePath string) bool {
	return strings.HasSuffix(filePath, ".java")
}
