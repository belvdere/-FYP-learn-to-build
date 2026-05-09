package python

import (
	"context"
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	pythonlang "github.com/smacker/go-tree-sitter/python"
)

// PythonParser wraps tree-sitter for parsing Python code.
type PythonParser struct {
	parser *sitter.Parser
}

// NewPythonParser creates a new Python parser instance.
func NewPythonParser() *PythonParser {
	p := sitter.NewParser()
	p.SetLanguage(pythonlang.GetLanguage())
	return &PythonParser{parser: p}
}

// ParseFile parses a Python source file and returns the AST and content.
func (p *PythonParser) ParseFile(filePath string) (*sitter.Tree, []byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read file: %w", err)
	}

	tree, err := p.parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, nil, fmt.Errorf("parse: %w", err)
	}

	return tree, content, nil
}

// Parse parses Python source code and returns the AST.
func (p *PythonParser) Parse(content []byte) (*sitter.Tree, error) {
	tree, err := p.parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return tree, nil
}

// Close releases parser resources.
func (p *PythonParser) Close() {
	p.parser.Close()
}
