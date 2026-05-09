package lang

import (
	"example.com/fyp/pkg/masking"
	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/segmentation"
	"example.com/fyp/pkg/validation"
	sitter "github.com/smacker/go-tree-sitter"
)

// ============================================================================
// Language Plugin Interface
// ============================================================================

// LanguagePlugin provides all language-specific implementations.
// Each language (Java, TypeScript, Python, etc.) implements this interface.
// This is the unified interface used by indexer, masking, and validation.
type LanguagePlugin interface {
	// Parsing
	Parse(content []byte) (*sitter.Tree, error)
	Parser() parser.Parser

	// Indexing components
	SymbolExtractor() parser.SymbolExtractor
	CallGraphExtractor() parser.CallGraphExtractor

	// Masking components
	Masker() masking.Masker
	Segmenter() segmentation.Segmenter

	// Validation
	Validator() validation.Validator

	// Metadata
	Language() string
	Extensions() []string

	// Detection
	DetectLanguage(content []byte, filePath string) bool
}
