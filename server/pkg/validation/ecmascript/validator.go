package ecmascript

import (
	"context"
	"fmt"
	"strings"

	"example.com/fyp/pkg/validation"
	sitter "github.com/smacker/go-tree-sitter"
	jslang "github.com/smacker/go-tree-sitter/javascript"
	tsxlang "github.com/smacker/go-tree-sitter/typescript/tsx"
)

// ECMAScriptValidator provides parse-stage validation for JS/TS.
type ECMAScriptValidator struct {
	parser *sitter.Parser
}

// NewJavaScriptValidator creates a JS validator.
func NewJavaScriptValidator() *ECMAScriptValidator {
	p := sitter.NewParser()
	p.SetLanguage(jslang.GetLanguage())
	return &ECMAScriptValidator{parser: p}
}

// NewTypeScriptValidator creates a TS/TSX validator.
func NewTypeScriptValidator() *ECMAScriptValidator {
	p := sitter.NewParser()
	p.SetLanguage(tsxlang.GetLanguage())
	return &ECMAScriptValidator{parser: p}
}

// ValidateFilledCode validates filled code at parse stage.
func (v *ECMAScriptValidator) ValidateFilledCode(filePath string, code string) ([]validation.ValidationResult, error) {
	return []validation.ValidationResult{v.validateParse(code)}, nil
}

func (v *ECMAScriptValidator) validateParse(code string) validation.ValidationResult {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return validation.ValidationResult{
			Stage:  validation.StageParse,
			Passed: false,
			Errors: []validation.ValidationError{{
				Line: 0, Column: 0, Severity: "error", Code: "PARSE_ERROR",
				Message: "Empty code cannot be parsed",
			}},
			Hints: []string{"Code must contain valid JavaScript/TypeScript syntax"},
		}
	}

	tree, err := v.parser.ParseCtx(context.Background(), nil, []byte(code))
	if err != nil {
		return validation.ValidationResult{
			Stage:  validation.StageParse,
			Passed: false,
			Errors: []validation.ValidationError{{
				Line: 0, Column: 0, Severity: "error", Code: "PARSE_ERROR",
				Message: fmt.Sprintf("Parse error: %v", err),
			}},
			Hints: []string{"Check syntax and unmatched delimiters"},
		}
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		errNode := findFirstErrorNode(root)
		line := 1
		col := 0
		if errNode != nil {
			line = int(errNode.StartPoint().Row) + 1
			col = int(errNode.StartPoint().Column)
		}
		snippet := extractLineSnippet(code, line)
		msg := fmt.Sprintf("Syntax error near line %d:%d", line, col)
		if snippet != "" {
			msg = fmt.Sprintf("Syntax error near line %d:%d: %s", line, col, snippet)
		}
		return validation.ValidationResult{
			Stage:  validation.StageParse,
			Passed: false,
			Errors: []validation.ValidationError{{
				Line: line, Column: col, Severity: "error", Code: "SYNTAX_ERROR", Message: msg,
			}},
			Hints: []string{"The code has syntax errors that prevent parsing"},
		}
	}

	return validation.ValidationResult{
		Stage:  validation.StageParse,
		Passed: true,
		Errors: []validation.ValidationError{},
		Hints:  []string{},
	}
}

func findFirstErrorNode(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	if node.Type() == "ERROR" {
		return node
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "ERROR" {
			return child
		}
		if child.HasError() {
			if found := findFirstErrorNode(child); found != nil {
				return found
			}
		}
	}
	if node.HasError() {
		return node
	}
	return nil
}

func extractLineSnippet(code string, line int) string {
	if line <= 0 {
		return ""
	}
	lines := strings.Split(code, "\n")
	if line > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[line-1])
}
