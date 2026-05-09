package java

import (
	"context"
	"fmt"
	"strings"

	"example.com/fyp/pkg/validation"
	sitter "github.com/smacker/go-tree-sitter"
	javaparser "github.com/smacker/go-tree-sitter/java"
)

// JavaValidator implements validation.Validator for Java code
// Performs parse-only validation using tree-sitter.
type JavaValidator struct {
	parser *sitter.Parser
}

// NewJavaValidator creates a new Java validator
func NewJavaValidator() *JavaValidator {
	parser := sitter.NewParser()
	parser.SetLanguage(javaparser.GetLanguage())

	return &JavaValidator{
		parser: parser,
	}
}

// ValidateFilledCode implements validation.Validator
// Runs parse-only validation using tree-sitter.
func (v *JavaValidator) ValidateFilledCode(
	filePath string,
	code string,
) ([]validation.ValidationResult, error) {
	results := []validation.ValidationResult{}

	// Parse validation (blocking) - must pass
	parseResult := v.validateParse(code)
	results = append(results, parseResult)

	return results, nil
}

// validateParse checks syntax using tree-sitter
func (v *JavaValidator) validateParse(code string) validation.ValidationResult {
	// Check for empty code
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
			Hints: []string{"Code must contain valid Java syntax"},
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
			Hints: []string{"Check for syntax errors, missing brackets, or invalid Java syntax"},
		}
	}
	defer tree.Close()

	// Check for parse errors in the tree
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
			// Snippet unavailable (e.g. line number out of range), but still include position.
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

	// Tree-sitter uses explicit ERROR nodes, but we also check HasError().
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

	// Fallback: return the node itself if it reports errors but no explicit ERROR child found.
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

