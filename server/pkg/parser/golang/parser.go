package golang

import (
	"context"
	"fmt"
	"os"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// GoParser wraps tree-sitter for parsing Go code.
type GoParser struct {
	parser *sitter.Parser
}

// NewGoParser creates a new Go parser instance.
func NewGoParser() *GoParser {
	p := sitter.NewParser()
	p.SetLanguage(golang.GetLanguage())

	return &GoParser{
		parser: p,
	}
}

// ParseFile parses a Go source file and returns the AST.
func (p *GoParser) ParseFile(filePath string) (*sitter.Tree, []byte, error) {
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

// Parse parses Go source code and returns the AST.
func (p *GoParser) Parse(content []byte) (*sitter.Tree, error) {
	tree, err := p.parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	return tree, nil
}

// Close releases resources used by the parser.
func (p *GoParser) Close() {
	p.parser.Close()
}
