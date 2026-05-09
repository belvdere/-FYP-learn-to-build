package ecmascript

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	jslang "github.com/smacker/go-tree-sitter/javascript"
	tsxlang "github.com/smacker/go-tree-sitter/typescript/tsx"
	tslang "github.com/smacker/go-tree-sitter/typescript/typescript"
)

// Variant identifies which ECMAScript family grammar set to use.
type Variant string

const (
	VariantJavaScript Variant = "javascript"
	VariantTypeScript Variant = "typescript"
)

// ECMAScriptParser wraps tree-sitter parsers for JS/TS/TSX.
type ECMAScriptParser struct {
	variant   Variant
	jsParser  *sitter.Parser
	tsParser  *sitter.Parser
	tsxParser *sitter.Parser
}

// NewJavaScriptParser creates a parser for JavaScript-family files.
func NewJavaScriptParser() *ECMAScriptParser {
	js := sitter.NewParser()
	js.SetLanguage(jslang.GetLanguage())
	return &ECMAScriptParser{
		variant:  VariantJavaScript,
		jsParser: js,
	}
}

// NewTypeScriptParser creates a parser for TypeScript-family files.
func NewTypeScriptParser() *ECMAScriptParser {
	ts := sitter.NewParser()
	ts.SetLanguage(tslang.GetLanguage())

	tsx := sitter.NewParser()
	tsx.SetLanguage(tsxlang.GetLanguage())

	return &ECMAScriptParser{
		variant:   VariantTypeScript,
		tsParser:  ts,
		tsxParser: tsx,
	}
}

// ParseFile parses a source file and chooses TS/TSX parser by extension when needed.
func (p *ECMAScriptParser) ParseFile(filePath string) (*sitter.Tree, []byte, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read file: %w", err)
	}
	tree, err := p.ParseByPath(content, filePath)
	if err != nil {
		return nil, nil, err
	}
	return tree, content, nil
}

// Parse parses source code with the parser's default grammar.
func (p *ECMAScriptParser) Parse(content []byte) (*sitter.Tree, error) {
	switch p.variant {
	case VariantJavaScript:
		return parseWith(p.jsParser, content)
	case VariantTypeScript:
		// TSX grammar accepts TS syntax and also JSX/TSX.
		return parseWith(p.tsxParser, content)
	default:
		return nil, fmt.Errorf("unsupported ECMAScript variant: %s", p.variant)
	}
}

// ParseByPath parses source code selecting grammar from file extension.
func (p *ECMAScriptParser) ParseByPath(content []byte, filePath string) (*sitter.Tree, error) {
	if p.variant == VariantJavaScript {
		return parseWith(p.jsParser, content)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".tsx":
		return parseWith(p.tsxParser, content)
	default:
		return parseWith(p.tsParser, content)
	}
}

// Close releases parser resources.
func (p *ECMAScriptParser) Close() {
	if p.jsParser != nil {
		p.jsParser.Close()
	}
	if p.tsParser != nil {
		p.tsParser.Close()
	}
	if p.tsxParser != nil {
		p.tsxParser.Close()
	}
}

func parseWith(parser *sitter.Parser, content []byte) (*sitter.Tree, error) {
	if parser == nil {
		return nil, fmt.Errorf("parser not initialized")
	}
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return tree, nil
}
