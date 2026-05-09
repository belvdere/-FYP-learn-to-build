package golang

import (
	"context"
	"fmt"
	"strings"

	"example.com/fyp/pkg/validation"
	sitter "github.com/smacker/go-tree-sitter"
	goparser "github.com/smacker/go-tree-sitter/golang"
)

// GoValidator implements validation.Validator for Go code.
// Performs parse-only validation using tree-sitter.
type GoValidator struct {
	parser *sitter.Parser
}

// NewGoValidator creates a new Go validator.
func NewGoValidator() *GoValidator {
	parser := sitter.NewParser()
	parser.SetLanguage(goparser.GetLanguage())
	return &GoValidator{parser: parser}
}

// ValidateFilledCode implements validation.Validator.
func (v *GoValidator) ValidateFilledCode(filePath string, code string) ([]validation.ValidationResult, error) {
	results := []validation.ValidationResult{}
	parseResult := v.validateParse(code)
	results = append(results, parseResult)
	return results, nil
}

// validateParse checks syntax using tree-sitter.
func (v *GoValidator) validateParse(code string) validation.ValidationResult {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return validation.ValidationResult{
			Stage:  validation.StageParse,
			Passed: false,
			Errors: []validation.ValidationError{
				{
					Line:     0,
					Column:   0,
					Message:  "Empty code cannot be parsed",
					Severity: "error",
					Code:     "PARSE_ERROR",
				},
			},
			Hints: []string{"Code must contain valid Go syntax"},
		}
	}

	ctx := context.Background()
	tree, err := v.parser.ParseCtx(ctx, nil, []byte(code))
	if err != nil {
		return validation.ValidationResult{
			Stage:  validation.StageParse,
			Passed: false,
			Errors: []validation.ValidationError{
				{
					Line:     0,
					Column:   0,
					Message:  fmt.Sprintf("Parse error: %v", err),
					Severity: "error",
					Code:     "PARSE_ERROR",
				},
			},
			Hints: []string{"Check for missing braces, unmatched parentheses, or invalid Go syntax"},
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
		var message string
		if snippet != "" {
			message = fmt.Sprintf("Syntax error near line %d:%d: %s", line, col, snippet)
		} else {
			message = fmt.Sprintf("Syntax error near line %d:%d", line, col)
		}
		return validation.ValidationResult{
			Stage:  validation.StageParse,
			Passed: false,
			Errors: []validation.ValidationError{
				{
					Line:     line,
					Column:   col,
					Message:  message,
					Severity: "error",
					Code:     "SYNTAX_ERROR",
				},
			},
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
