package parser

import (
	"example.com/fyp/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
)

// ============================================================================
// Core Parser Interface
// ============================================================================

// Parser is the interface for language-specific parsers.
type Parser interface {
	// ParseFile parses a source file and returns the AST and content.
	ParseFile(filePath string) (*sitter.Tree, []byte, error)

	// Parse parses source code and returns the AST.
	Parse(content []byte) (*sitter.Tree, error)

	// Close releases resources used by the parser.
	Close()
}

// ============================================================================
// Symbol Extraction Interface
// ============================================================================

// SymbolExtractor extracts symbols (classes, functions, etc.) from parsed code.
type SymbolExtractor interface {
	// ExtractSymbols extracts all symbols from a parsed file.
	ExtractSymbols(tree *sitter.Tree, content []byte, filePath string) []types.Symbol
}

// ============================================================================
// Call Graph Extraction Interface
// ============================================================================

// CallGraphExtractor extracts method/function call relationships.
type CallGraphExtractor interface {
	// ExtractMethodCalls extracts all method invocations from a file.
	ExtractMethodCalls(filePath string, sourceCode []byte) ([]types.MethodCall, error)
}

// ============================================================================
// Helper Functions (Language-Agnostic)
// ============================================================================

// GetNodeText extracts text for a node from source content.
func GetNodeText(node *sitter.Node, content []byte) string {
	return string(content[node.StartByte():node.EndByte()])
}

// FindNodesByType finds all nodes of a specific type in the tree.
func FindNodesByType(root *sitter.Node, nodeType string) []*sitter.Node {
	var results []*sitter.Node

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		if node.Type() == nodeType {
			results = append(results, node)
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)
	return results
}

// GetParentOfType finds the closest parent node of a specific type.
func GetParentOfType(node *sitter.Node, nodeType string) *sitter.Node {
	current := node.Parent()
	for current != nil {
		if current.Type() == nodeType {
			return current
		}
		current = current.Parent()
	}
	return nil
}

// FindChildByType finds the first direct child node of a specific type.
func FindChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == nodeType {
			return child
		}
	}
	return nil
}
