package ecmascript

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"example.com/fyp/pkg/db"
	"example.com/fyp/pkg/masking"
	"github.com/google/uuid"
	sitter "github.com/smacker/go-tree-sitter"
	jslang "github.com/smacker/go-tree-sitter/javascript"
	tsxlang "github.com/smacker/go-tree-sitter/typescript/tsx"
)

// ECMAScriptMasker masks JavaScript/TypeScript decision points.
type ECMAScriptMasker struct {
	parser   *sitter.Parser
	config   *masking.MaskingConfig
	language string
}

// NewJavaScriptMasker creates a JS masker.
func NewJavaScriptMasker() *ECMAScriptMasker {
	p := sitter.NewParser()
	p.SetLanguage(jslang.GetLanguage())
	return &ECMAScriptMasker{
		parser:   p,
		config:   masking.DefaultMaskingConfig(),
		language: "javascript",
	}
}

// NewTypeScriptMasker creates a TS/TSX masker.
func NewTypeScriptMasker() *ECMAScriptMasker {
	p := sitter.NewParser()
	p.SetLanguage(tsxlang.GetLanguage())
	return &ECMAScriptMasker{
		parser:   p,
		config:   masking.DefaultMaskingConfig(),
		language: "typescript",
	}
}

// MaskCode inserts [MASK] markers while keeping syntax parseable.
func (m *ECMAScriptMasker) MaskCode(code string) (*masking.MaskResult, error) {
	tree, err := m.parser.ParseCtx(context.Background(), nil, []byte(code))
	if err != nil {
		return nil, fmt.Errorf("parse code: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		return nil, fmt.Errorf("code contains syntax errors")
	}

	points := m.identifyDecisionPoints(root, []byte(code))
	sort.Slice(points, func(i, j int) bool {
		return points[i].Priority > points[j].Priority
	})
	if m.config.MaxMasksPerFunction > 0 && len(points) > m.config.MaxMasksPerFunction {
		points = points[:m.config.MaxMasksPerFunction]
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].StartByte > points[j].StartByte
	})

	maskedCode := code
	masks := make([]masking.Mask, 0, len(points))
	for _, p := range points {
		mask, next := m.insertMaskMarker(maskedCode, p)
		maskedCode = next
		masks = append(masks, mask)
	}

	return &masking.MaskResult{
		MaskedCode: maskedCode,
		Masks:      masks,
	}, nil
}

// MaskCodeWithStore masks code and stores session in DB.
func (m *ECMAScriptMasker) MaskCodeWithStore(code string, filePath string, store *db.Store) (string, string, error) {
	result, err := m.MaskCode(code)
	if err != nil {
		return "", "", fmt.Errorf("mask code: %w", err)
	}

	sessionID := uuid.New().String()
	maskedCode := strings.ReplaceAll(
		result.MaskedCode,
		"/* [MASK:id=",
		fmt.Sprintf("/* [MASK:session=%s id=", sessionID),
	)

	session := &db.MaskSession{
		ID:           sessionID,
		FilePath:     filePath,
		OriginalCode: code,
		MaskedCode:   maskedCode,
		Language:     m.language,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Status:       db.StatusPending,
	}
	if err := store.CreateMaskSession(session); err != nil {
		return "", "", fmt.Errorf("store session: %w", err)
	}
	return maskedCode, sessionID, nil
}

func (m *ECMAScriptMasker) identifyDecisionPoints(root *sitter.Node, content []byte) []masking.DecisionPoint {
	points := make([]masking.DecisionPoint, 0, 12)

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "if_statement", "while_statement", "for_statement", "do_statement":
			if m.config.MaskValidation {
				if p := analyzeConditionNode(node, content, m.config); p != nil {
					points = append(points, *p)
				}
			}
		case "switch_statement", "switch_expression":
			if m.config.MaskValidation {
				if p := analyzeSwitchNode(node, content, m.config); p != nil {
					points = append(points, *p)
				}
			}
		case "ternary_expression":
			if m.config.MaskValidation {
				if p := analyzeConditionNode(node, content, m.config); p != nil {
					points = append(points, *p)
				}
			}
		case "return_statement":
			if m.config.MaskReturns {
				if p := analyzeReturnNode(node, content); p != nil {
					points = append(points, *p)
				}
			}
		case "throw_statement":
			if m.config.MaskErrors {
				if p := analyzeThrowNode(node, content); p != nil {
					points = append(points, *p)
				}
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)
	return points
}

func analyzeConditionNode(node *sitter.Node, content []byte, cfg *masking.MaskingConfig) *masking.DecisionPoint {
	cond := node.ChildByFieldName("condition")
	if cond == nil {
		return nil
	}

	original := string(content[cond.StartByte():cond.EndByte()])
	complexity := calculateComplexity(original)
	if complexity < cfg.ComplexityThreshold {
		return nil
	}

	maskType := masking.MaskTypeCondition
	hint := "complete the condition"
	if isValidationPattern(original) {
		maskType = masking.MaskTypeValidation
		hint = "add validation check"
	}

	return &masking.DecisionPoint{
		Type:         maskType,
		Node:         cond,
		StartByte:    cond.StartByte(),
		EndByte:      cond.EndByte(),
		StartLine:    int(cond.StartPoint().Row),
		EndLine:      int(cond.EndPoint().Row),
		Hint:         hint,
		OriginalCode: original,
		Priority:     complexity * 10,
	}
}

func analyzeSwitchNode(node *sitter.Node, content []byte, cfg *masking.MaskingConfig) *masking.DecisionPoint {
	value := node.ChildByFieldName("value")
	if value == nil {
		value = node.ChildByFieldName("condition")
	}
	if value == nil {
		return nil
	}
	original := string(content[value.StartByte():value.EndByte()])
	complexity := calculateComplexity(original)
	if complexity < cfg.ComplexityThreshold {
		return nil
	}

	return &masking.DecisionPoint{
		Type:         masking.MaskTypeCondition,
		Node:         value,
		StartByte:    value.StartByte(),
		EndByte:      value.EndByte(),
		StartLine:    int(value.StartPoint().Row),
		EndLine:      int(value.EndPoint().Row),
		Hint:         "complete switch selector",
		OriginalCode: original,
		Priority:     complexity * 8,
	}
}

func analyzeReturnNode(node *sitter.Node, content []byte) *masking.DecisionPoint {
	original := string(content[node.StartByte():node.EndByte()])
	trimmed := strings.TrimSpace(original)
	if trimmed == "return;" {
		return nil
	}
	return &masking.DecisionPoint{
		Type:         masking.MaskTypeReturn,
		Node:         node,
		StartByte:    node.StartByte(),
		EndByte:      node.EndByte(),
		StartLine:    int(node.StartPoint().Row),
		EndLine:      int(node.EndPoint().Row),
		Hint:         "return the correct value",
		OriginalCode: original,
		Priority:     6,
	}
}

func analyzeThrowNode(node *sitter.Node, content []byte) *masking.DecisionPoint {
	original := string(content[node.StartByte():node.EndByte()])
	return &masking.DecisionPoint{
		Type:         masking.MaskTypeError,
		Node:         node,
		StartByte:    node.StartByte(),
		EndByte:      node.EndByte(),
		StartLine:    int(node.StartPoint().Row),
		EndLine:      int(node.EndPoint().Row),
		Hint:         "add error handling",
		OriginalCode: original,
		Priority:     7,
	}
}

func (m *ECMAScriptMasker) insertMaskMarker(code string, point masking.DecisionPoint) (masking.Mask, string) {
	maskID := generateMaskID()
	comment := fmt.Sprintf("/* [MASK:id=%s hint=\"%s\"] */", maskID, point.Hint)

	placeholder := comment
	switch point.Type {
	case masking.MaskTypeCondition, masking.MaskTypeValidation:
		placeholder = "true " + comment
	case masking.MaskTypeReturn:
		placeholder = "return null; " + comment
	case masking.MaskTypeError:
		placeholder = "throw new Error(\"TODO\"); " + comment
	}

	before := code[:point.StartByte]
	after := code[point.EndByte:]
	maskedCode := before + placeholder + after

	mask := masking.Mask{
		ID:           maskID,
		Type:         point.Type,
		Hint:         point.Hint,
		OriginalCode: point.OriginalCode,
		Range: masking.Range{
			Start: byteToPosition(code, int(point.StartByte)),
			End:   byteToPosition(code, int(point.EndByte)),
		},
	}

	return mask, maskedCode
}

func byteToPosition(code string, byteOffset int) masking.Position {
	if byteOffset > len(code) {
		byteOffset = len(code)
	}
	line := 0
	char := 0
	for _, r := range code[:byteOffset] {
		if r == '\n' {
			line++
			char = 0
			continue
		}
		char++
	}
	return masking.Position{Line: line, Character: char}
}

func calculateComplexity(code string) int {
	complexity := 0
	complexity += strings.Count(code, "&&")
	complexity += strings.Count(code, "||")
	complexity += strings.Count(code, "===")
	complexity += strings.Count(code, "!==")
	// Count == and != only when not already counted as part of === / !==
	stripped := strings.ReplaceAll(strings.ReplaceAll(code, "===", "   "), "!==", "   ")
	complexity += strings.Count(stripped, "==")
	complexity += strings.Count(stripped, "!=")
	complexity += strings.Count(code, ">")
	complexity += strings.Count(code, "<")
	if complexity == 0 && strings.TrimSpace(code) != "" {
		complexity = 1
	}
	return complexity
}

func isValidationPattern(code string) bool {
	c := strings.ToLower(code)
	keywords := []string{"null", "undefined", "length", "empty", "validate", "check", "instanceof"}
	for _, kw := range keywords {
		if strings.Contains(c, kw) {
			return true
		}
	}
	return false
}

func generateMaskID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "mask_" + uuid.New().String()
	}
	return "mask_" + hex.EncodeToString(bytes)
}
