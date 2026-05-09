package java

import (
	"context"
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

// JavaParser wraps tree-sitter for parsing Java code.
type JavaParser struct {
	parser *sitter.Parser
}

// NewJavaParser creates a new Java parser instance.
func NewJavaParser() *JavaParser {
	p := sitter.NewParser()
	p.SetLanguage(java.GetLanguage())

	return &JavaParser{
		parser: p,
	}
}

// ParseFile parses a Java source file and returns the AST.
func (p *JavaParser) ParseFile(filePath string) (*sitter.Tree, []byte, error) {
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

// Parse parses Java source code and returns the AST.
func (p *JavaParser) Parse(content []byte) (*sitter.Tree, error) {
	tree, err := p.parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	return tree, nil
}

// Close releases resources used by the parser.
func (p *JavaParser) Close() {
	p.parser.Close()
}
