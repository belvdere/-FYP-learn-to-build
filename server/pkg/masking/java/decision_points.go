package java

import (
	"strings"

	"example.com/fyp/pkg/masking"
	sitter "github.com/smacker/go-tree-sitter"
)

// analyzeIfStatement analyzes an if statement for masking.
func analyzeIfStatement(node *sitter.Node, content []byte, config *masking.MaskingConfig) *masking.DecisionPoint {
	// Get the condition node
	conditionNode := node.ChildByFieldName("condition")
	if conditionNode == nil {
		return nil
	}

	conditionCode := string(content[conditionNode.StartByte():conditionNode.EndByte()])

	// Calculate complexity
	complexity := calculateComplexity(conditionCode)
	if complexity < config.ComplexityThreshold {
		return nil // Skip simple conditions
	}

	// Determine if this is validation; otherwise treat as a general condition
	maskType := masking.MaskTypeCondition
	hint := "complete the condition"

	if isValidationPattern(conditionCode) {
		maskType = masking.MaskTypeValidation
		hint = "add validation check"
	}

	return &masking.DecisionPoint{
		Type:         maskType,
		Node:         conditionNode,
		StartByte:    conditionNode.StartByte(),
		EndByte:      conditionNode.EndByte(),
		StartLine:    int(conditionNode.StartPoint().Row),
		EndLine:      int(conditionNode.EndPoint().Row),
		Hint:         hint,
		OriginalCode: conditionCode,
		Priority:     complexity * 10, // Higher complexity = higher priority
	}
}

// analyzeReturnStatement analyzes a return statement for masking.
func analyzeReturnStatement(node *sitter.Node, content []byte, config *masking.MaskingConfig) *masking.DecisionPoint {
	// Only mask returns with complex expressions
	returnCode := string(content[node.StartByte():node.EndByte()])

	// Skip simple returns like "return;" or "return null;"
	if strings.Contains(returnCode, "return;") || strings.Contains(returnCode, "return null;") {
		return nil
	}

	complexity := calculateComplexity(returnCode)
	if complexity < config.ComplexityThreshold {
		return nil
	}

	return &masking.DecisionPoint{
		Type:         masking.MaskTypeReturn,
		Node:         node,
		StartByte:    node.StartByte(),
		EndByte:      node.EndByte(),
		StartLine:    int(node.StartPoint().Row),
		EndLine:      int(node.EndPoint().Row),
		Hint:         "complete the return statement",
		OriginalCode: returnCode,
		Priority:     complexity * 8,
	}
}

// analyzeThrowStatement analyzes a throw statement for masking.
func analyzeThrowStatement(node *sitter.Node, content []byte, config *masking.MaskingConfig) *masking.DecisionPoint {
	throwCode := string(content[node.StartByte():node.EndByte()])

	return &masking.DecisionPoint{
		Type:         masking.MaskTypeError,
		Node:         node,
		StartByte:    node.StartByte(),
		EndByte:      node.EndByte(),
		StartLine:    int(node.StartPoint().Row),
		EndLine:      int(node.EndPoint().Row),
		Hint:         "add error handling",
		OriginalCode: throwCode,
		Priority:     7, // Medium priority
	}
}

// calculateComplexity estimates code complexity
func calculateComplexity(code string) int {
	complexity := 0

	// Count operators
	complexity += strings.Count(code, "&&")
	complexity += strings.Count(code, "||")
	complexity += strings.Count(code, "==")
	complexity += strings.Count(code, "!=")
	complexity += strings.Count(code, ">")
	complexity += strings.Count(code, "<")

	// Count method calls
	complexity += strings.Count(code, "(") / 2

	// Count nested access
	complexity += strings.Count(code, ".")

	return complexity
}

// isValidationPattern checks if code looks like validation
func isValidationPattern(code string) bool {
	validationKeywords := []string{
		"null", "isEmpty", "isBlank", "length",
		"valid", "check", "verify", "assert",
	}

	codeLower := strings.ToLower(code)
	for _, keyword := range validationKeywords {
		if strings.Contains(codeLower, keyword) {
			return true
		}
	}

	return false
}

// analyzeSwitchStatement analyzes a switch statement for masking.
func analyzeSwitchStatement(node *sitter.Node, content []byte, config *masking.MaskingConfig) *masking.DecisionPoint {
	// Get the switch expression (the value being switched on)
	// In tree-sitter, the switch expression is typically the first child or a field named "condition"
	var expressionNode *sitter.Node

	// Try to find the expression node
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		// The expression is usually before the switch body
		if child.Type() == "parenthesized_expression" ||
			child.Type() == "identifier" ||
			child.Type() == "field_access" ||
			child.Type() == "method_invocation" {
			expressionNode = child
			break
		}
	}

	// Fallback: use the condition field if available
	if expressionNode == nil {
		expressionNode = node.ChildByFieldName("condition")
	}

	// If still not found, try the first child that's not a keyword
	if expressionNode == nil {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() != "switch" && child.Type() != "{" {
				expressionNode = child
				break
			}
		}
	}

	if expressionNode == nil {
		return nil
	}

	expressionCode := string(content[expressionNode.StartByte():expressionNode.EndByte()])

	// Calculate complexity
	complexity := calculateComplexity(expressionCode)

	// Switch statements are decision points, so we should mask them even if simple
	// But we can still use complexity for priority
	if complexity < 1 {
		complexity = 1 // Minimum complexity for switch
	}

	// Determine mask type
	maskType := masking.MaskTypeCondition
	hint := "complete the switch expression"

	if isValidationPattern(expressionCode) {
		maskType = masking.MaskTypeValidation
		hint = "add validation check"
	}

	return &masking.DecisionPoint{
		Type:         maskType,
		Node:         expressionNode,
		StartByte:    expressionNode.StartByte(),
		EndByte:      expressionNode.EndByte(),
		StartLine:    int(expressionNode.StartPoint().Row),
		EndLine:      int(expressionNode.EndPoint().Row),
		Hint:         hint,
		OriginalCode: expressionCode,
		Priority:     complexity * 9, // Slightly lower than if statements
	}
}

// analyzeTernaryExpression analyzes a ternary operator (condition ? trueValue : falseValue) for masking.
func analyzeTernaryExpression(node *sitter.Node, content []byte, config *masking.MaskingConfig) *masking.DecisionPoint {
	// Ternary operator has three parts: condition, consequence, alternative
	// We want to mask the condition part
	var conditionNode *sitter.Node

	// Try to find the condition (first part before ?)
	// In tree-sitter, ternary expressions are typically "conditional_expression"
	// The condition is usually the first child
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		// The condition is before the "?" token
		if child.Type() != "?" && child.Type() != ":" {
			conditionNode = child
			break
		}
	}

	// Fallback: use the condition field if available
	if conditionNode == nil {
		conditionNode = node.ChildByFieldName("condition")
	}

	// If still not found, use the first non-operator child
	if conditionNode == nil {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() != "?" && child.Type() != ":" &&
				child.Type() != "ternary_expression" &&
				child.Type() != "conditional_expression" {
				conditionNode = child
				break
			}
		}
	}

	if conditionNode == nil {
		return nil
	}

	conditionCode := string(content[conditionNode.StartByte():conditionNode.EndByte()])

	// Calculate complexity
	complexity := calculateComplexity(conditionCode)

	// Ternary operators are decision points, mask them even if simple
	if complexity < 1 {
		complexity = 1 // Minimum complexity for ternary
	}

	// Determine mask type
	maskType := masking.MaskTypeCondition
	hint := "complete the condition"

	if isValidationPattern(conditionCode) {
		maskType = masking.MaskTypeValidation
		hint = "add validation check"
	}

	return &masking.DecisionPoint{
		Type:         maskType,
		Node:         conditionNode,
		StartByte:    conditionNode.StartByte(),
		EndByte:      conditionNode.EndByte(),
		StartLine:    int(conditionNode.StartPoint().Row),
		EndLine:      int(conditionNode.EndPoint().Row),
		Hint:         hint,
		OriginalCode: conditionCode,
		Priority:     complexity * 8, // Lower priority than if/switch
	}
}
