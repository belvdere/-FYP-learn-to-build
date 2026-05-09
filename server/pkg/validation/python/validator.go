package python

import (
	"context"
	"fmt"
	"strings"

	"example.com/fyp/pkg/validation"
	sitter "github.com/smacker/go-tree-sitter"
	pythonlang "github.com/smacker/go-tree-sitter/python"
)

// PythonValidator validates Python code with parse-stage checks.
type PythonValidator struct {
	parser *sitter.Parser
}

// NewPythonValidator creates a Python validator.
func NewPythonValidator() *PythonValidator {
	p := sitter.NewParser()
	p.SetLanguage(pythonlang.GetLanguage())
	return &PythonValidator{parser: p}
}

// ValidateFilledCode implements validation.Validator.
func (v *PythonValidator) ValidateFilledCode(filePath string, code string) ([]validation.ValidationResult, error) {
	return []validation.ValidationResult{v.validateParse(code)}, nil
}

func (v *PythonValidator) validateParse(code string) validation.ValidationResult {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return validation.ValidationResult{
			Stage:  validation.StageParse,
			Passed: false,
			Errors: []validation.ValidationError{{
				Line: 0, Column: 0, Severity: "error", Code: "PARSE_ERROR",
				Message: "Empty code cannot be parsed",
			}},
			Hints: []string{"Code must contain valid Python syntax"},
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
			Hints: []string{"Check Python syntax and indentation"},
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
			Hints: []string{"The code has syntax or indentation errors"},
		}
	}

	return validation.ValidationResult{Stage: validation.StageParse, Passed: true, Errors: []validation.ValidationError{}, Hints: []string{}}
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
