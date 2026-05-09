package golang

import (
	"strings"

	"example.com/fyp/pkg/masking"
	sitter "github.com/smacker/go-tree-sitter"
)

// analyzeGoIfStatement analyzes a Go if statement for masking.
func analyzeGoIfStatement(node *sitter.Node, content []byte, config *masking.MaskingConfig) *masking.DecisionPoint {
	conditionNode := node.ChildByFieldName("condition")
	if conditionNode == nil {
		return nil
	}

	conditionCode := string(content[conditionNode.StartByte():conditionNode.EndByte()])

	// Error check: if err != nil is always masked (priority 15)
	if isGoErrorCheck(conditionCode) {
		return &masking.DecisionPoint{
			Type:         masking.MaskTypeValidation,
			Node:         conditionNode,
			StartByte:    conditionNode.StartByte(),
			EndByte:      conditionNode.EndByte(),
			StartLine:    int(conditionNode.StartPoint().Row),
			EndLine:      int(conditionNode.EndPoint().Row),
			Hint:         "add error check",
			OriginalCode: conditionCode,
			Priority:     15,
		}
	}

	complexity := calculateGoComplexity(conditionCode)
	if complexity < config.ComplexityThreshold {
		return nil
	}

	maskType := masking.MaskTypeCondition
	hint := "complete the condition"

	if isGoValidationPattern(conditionCode) {
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
		Priority:     complexity * 10,
	}
}

// analyzeGoReturnStatement analyzes a Go return statement for masking.
func analyzeGoReturnStatement(node *sitter.Node, content []byte, config *masking.MaskingConfig) *masking.DecisionPoint {
	returnCode := string(content[node.StartByte():node.EndByte()])

	// Skip bare "return" or "return nil"
	trimmed := strings.TrimSpace(returnCode)
	if trimmed == "return" || trimmed == "return nil" {
		return nil
	}

	complexity := calculateGoComplexity(returnCode)
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

// analyzeGoSwitchStatement analyzes a Go expression switch statement for masking.
func analyzeGoSwitchStatement(node *sitter.Node, content []byte, config *masking.MaskingConfig) *masking.DecisionPoint {
	// In Go tree-sitter: expression_switch_statement has optional "value" field
	valueNode := node.ChildByFieldName("value")
	if valueNode == nil {
		// Bare switch {} (condition-less) — find first non-keyword child
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil && child.Type() != "switch" && child.Type() != "{" && child.Type() != "}" {
				valueNode = child
				break
			}
		}
	}

	if valueNode == nil {
		return nil
	}

	expressionCode := string(content[valueNode.StartByte():valueNode.EndByte()])
	complexity := calculateGoComplexity(expressionCode)
	if complexity < 1 {
		complexity = 1
	}

	maskType := masking.MaskTypeCondition
	hint := "complete the switch expression"

	if isGoValidationPattern(expressionCode) {
		maskType = masking.MaskTypeValidation
		hint = "add validation check"
	}

	return &masking.DecisionPoint{
		Type:         maskType,
		Node:         valueNode,
		StartByte:    valueNode.StartByte(),
		EndByte:      valueNode.EndByte(),
		StartLine:    int(valueNode.StartPoint().Row),
		EndLine:      int(valueNode.EndPoint().Row),
		Hint:         hint,
		OriginalCode: expressionCode,
		Priority:     complexity * 9,
	}
}

// isGoErrorCheck returns true when the condition is a Go error check: err != nil
func isGoErrorCheck(code string) bool {
	return strings.Contains(code, "err") && strings.Contains(code, "!=")
}

// calculateGoComplexity estimates code complexity for Go.
func calculateGoComplexity(code string) int {
	complexity := 0
	complexity += strings.Count(code, "&&")
	complexity += strings.Count(code, "||")
	complexity += strings.Count(code, "==")
	complexity += strings.Count(code, "!=")
	complexity += strings.Count(code, ">")
	complexity += strings.Count(code, "<")
	complexity += strings.Count(code, "(") / 2
	complexity += strings.Count(code, ".")
	return complexity
}

// isGoValidationPattern checks if code looks like a validation/nil check in Go.
func isGoValidationPattern(code string) bool {
	validationKeywords := []string{
		"nil", "err", "empty", "valid", "check",
		"null", "isEmpty", "isBlank", "length",
		"verify", "assert",
	}
	codeLower := strings.ToLower(code)
	for _, kw := range validationKeywords {
		if strings.Contains(codeLower, kw) {
			return true
		}
	}
	return false
}
