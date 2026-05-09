package python

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
	pythonlang "github.com/smacker/go-tree-sitter/python"
)

// PythonMasker implements Masker for Python.
type PythonMasker struct {
	parser *sitter.Parser
	config *masking.MaskingConfig
}

// NewPythonMasker creates a Python masker instance.
func NewPythonMasker() *PythonMasker {
	p := sitter.NewParser()
	p.SetLanguage(pythonlang.GetLanguage())
	return &PythonMasker{parser: p, config: masking.DefaultMaskingConfig()}
}

// MaskCode analyzes Python code and inserts [MASK] markers.
func (m *PythonMasker) MaskCode(code string) (*masking.MaskResult, error) {
	tree, err := m.parser.ParseCtx(context.Background(), nil, []byte(code))
	if err != nil {
		return nil, fmt.Errorf("parse code: %w", err)
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		return nil, fmt.Errorf("code contains syntax errors")
	}

	points := m.identifyDecisionPoints(tree.RootNode(), []byte(code))
	sort.Slice(points, func(i, j int) bool { return points[i].Priority > points[j].Priority })
	if m.config.MaxMasksPerFunction > 0 && len(points) > m.config.MaxMasksPerFunction {
		points = points[:m.config.MaxMasksPerFunction]
	}

	// Build mask metadata and collect all text replacement operations.
	// Each decision point generates two ops: comment insertion + span replacement.
	// We apply all ops sorted by descending start offset so that higher-offset
	// modifications never shift the byte positions used by lower-offset ones.
	type textOp struct {
		start int
		end   int
		text  string
	}
	masks := make([]masking.Mask, 0, len(points))
	ops := make([]textOp, 0, len(points)*2)

	for _, p := range points {
		maskID := generateMaskID()
		hint := strings.ReplaceAll(p.Hint, "\"", "'")
		comment := p.Indent + fmt.Sprintf("# [MASK:id=%s hint=\"%s\"]\n", maskID, hint)

		// Span replacement: [StartByte, EndByte) → Placeholder
		ops = append(ops, textOp{int(p.StartByte), int(p.EndByte), p.Placeholder})
		// Comment insertion: zero-width at CommentByte (before the statement line)
		ops = append(ops, textOp{int(p.CommentByte), int(p.CommentByte), comment})

		masks = append(masks, masking.Mask{
			ID:           maskID,
			Type:         p.Type,
			Hint:         p.Hint,
			OriginalCode: p.OriginalCode,
			Range: masking.Range{
				Start: byteToPosition(code, int(p.StartByte)),
				End:   byteToPosition(code, int(p.EndByte)),
			},
		})
	}

	// Apply all ops from highest offset to lowest so earlier ops don't shift later positions.
	sort.Slice(ops, func(i, j int) bool { return ops[i].start > ops[j].start })
	maskedCode := code
	for _, op := range ops {
		maskedCode = replaceSpan(maskedCode, op.start, op.end, op.text)
	}

	return &masking.MaskResult{MaskedCode: maskedCode, Masks: masks}, nil
}

// MaskCodeWithStore masks code and stores the session in DB.
func (m *PythonMasker) MaskCodeWithStore(code string, filePath string, store *db.Store) (string, string, error) {
	result, err := m.MaskCode(code)
	if err != nil {
		return "", "", fmt.Errorf("mask code: %w", err)
	}

	sessionID := uuid.New().String()
	maskedCode := strings.ReplaceAll(
		result.MaskedCode,
		"# [MASK:id=",
		fmt.Sprintf("# [MASK:session=%s id=", sessionID),
	)

	session := &db.MaskSession{
		ID:           sessionID,
		FilePath:     filePath,
		OriginalCode: code,
		MaskedCode:   maskedCode,
		Language:     "python",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Status:       db.StatusPending,
	}
	if err := store.CreateMaskSession(session); err != nil {
		return "", "", fmt.Errorf("store session: %w", err)
	}

	return maskedCode, sessionID, nil
}

type decisionPoint struct {
	masking.DecisionPoint
	CommentByte uint32
	Indent      string
	Placeholder string
}

func (m *PythonMasker) identifyDecisionPoints(root *sitter.Node, content []byte) []decisionPoint {
	points := make([]decisionPoint, 0, 8)

	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "if_statement", "elif_clause":
			if m.config.MaskValidation {
				if p := analyzeConditionNode(node, content, m.config); p != nil {
					points = append(points, *p)
				}
			}
		case "conditional_expression":
			if m.config.MaskValidation {
				if p := analyzeConditionalExpression(node, content, m.config); p != nil {
					points = append(points, *p)
				}
			}
		case "raise_statement":
			if m.config.MaskErrors {
				if p := analyzeRaiseStatement(node, content); p != nil {
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

func analyzeConditionNode(node *sitter.Node, content []byte, cfg *masking.MaskingConfig) *decisionPoint {
	cond := node.ChildByFieldName("condition")
	if cond == nil {
		return nil
	}
	code := string(content[cond.StartByte():cond.EndByte()])
	complexity := calculateComplexity(code)
	if complexity < cfg.ComplexityThreshold {
		return nil
	}

	maskType := masking.MaskTypeCondition
	hint := "complete the condition"
	if isValidationPattern(code) {
		maskType = masking.MaskTypeValidation
		hint = "add validation check"
	}

	statementStart := node.StartByte()
	lineStart, indent := lineStartAndIndent(content, int(statementStart))
	return &decisionPoint{
		DecisionPoint: masking.DecisionPoint{
			Type:         maskType,
			Node:         cond,
			StartByte:    cond.StartByte(),
			EndByte:      cond.EndByte(),
			StartLine:    int(cond.StartPoint().Row),
			EndLine:      int(cond.EndPoint().Row),
			Hint:         hint,
			OriginalCode: code,
			Priority:     complexity * 10,
		},
		CommentByte: uint32(lineStart),
		Indent:      indent,
		Placeholder: "True",
	}
}

func analyzeConditionalExpression(node *sitter.Node, content []byte, cfg *masking.MaskingConfig) *decisionPoint {
	cond := node.ChildByFieldName("condition")
	if cond == nil {
		if node.ChildCount() > 0 {
			cond = node.Child(0)
		}
	}
	if cond == nil {
		return nil
	}

	code := string(content[cond.StartByte():cond.EndByte()])
	complexity := calculateComplexity(code)
	if complexity < cfg.ComplexityThreshold {
		return nil
	}

	maskType := masking.MaskTypeCondition
	hint := "complete the condition"
	if isValidationPattern(code) {
		maskType = masking.MaskTypeValidation
		hint = "add validation check"
	}

	stmt := enclosingStatementNode(node)
	lineStart, indent := lineStartAndIndent(content, int(stmt.StartByte()))
	return &decisionPoint{
		DecisionPoint: masking.DecisionPoint{
			Type:         maskType,
			Node:         cond,
			StartByte:    cond.StartByte(),
			EndByte:      cond.EndByte(),
			StartLine:    int(cond.StartPoint().Row),
			EndLine:      int(cond.EndPoint().Row),
			Hint:         hint,
			OriginalCode: code,
			Priority:     complexity * 8,
		},
		CommentByte: uint32(lineStart),
		Indent:      indent,
		Placeholder: "True",
	}
}

func analyzeRaiseStatement(node *sitter.Node, content []byte) *decisionPoint {
	code := string(content[node.StartByte():node.EndByte()])
	lineStart, indent := lineStartAndIndent(content, int(node.StartByte()))
	return &decisionPoint{
		DecisionPoint: masking.DecisionPoint{
			Type:         masking.MaskTypeError,
			Node:         node,
			StartByte:    node.StartByte(),
			EndByte:      node.EndByte(),
			StartLine:    int(node.StartPoint().Row),
			EndLine:      int(node.EndPoint().Row),
			Hint:         "add error handling",
			OriginalCode: code,
			Priority:     7,
		},
		CommentByte: uint32(lineStart),
		Indent:      indent,
		Placeholder: `raise Exception("TODO")`,
	}
}

func enclosingStatementNode(node *sitter.Node) *sitter.Node {
	if node == nil {
		return node
	}
	statementTypes := map[string]bool{
		"expression_statement": true,
		"assignment":           true,
		"augmented_assignment": true,
		"return_statement":     true,
		"if_statement":         true,
		"elif_clause":          true,
		"while_statement":      true,
		"for_statement":        true,
		"with_statement":       true,
		"raise_statement":      true,
		"assert_statement":     true,
	}
	current := node
	for current.Parent() != nil {
		if statementTypes[current.Type()] {
			return current
		}
		current = current.Parent()
	}
	return node
}

func calculateComplexity(code string) int {
	complexity := 0
	complexity += strings.Count(code, " and ")
	complexity += strings.Count(code, " or ")
	complexity += strings.Count(code, "==")
	complexity += strings.Count(code, "!=")
	complexity += strings.Count(code, ">")
	complexity += strings.Count(code, "<")
	complexity += strings.Count(code, ".")
	if complexity == 0 && strings.TrimSpace(code) != "" {
		complexity = 1
	}
	return complexity
}

func isValidationPattern(code string) bool {
	c := strings.ToLower(code)
	keywords := []string{"none", "empty", "len(", "validate", "check", "assert", "isinstance", "not "}
	for _, kw := range keywords {
		if strings.Contains(c, kw) {
			return true
		}
	}
	return false
}

func lineStartAndIndent(content []byte, byteOffset int) (lineStart int, indent string) {
	if byteOffset < 0 {
		byteOffset = 0
	}
	if byteOffset > len(content) {
		byteOffset = len(content)
	}

	lineStart = 0
	for i := byteOffset - 1; i >= 0; i-- {
		if content[i] == '\n' {
			lineStart = i + 1
			break
		}
	}

	i := lineStart
	for i < len(content) {
		if content[i] == ' ' || content[i] == '\t' {
			i++
			continue
		}
		break
	}
	indent = string(content[lineStart:i])
	return lineStart, indent
}

func replaceSpan(s string, start, end int, replacement string) string {
	if start < 0 {
		start = 0
	}
	if end > len(s) {
		end = len(s)
	}
	if start > end {
		start = end
	}
	return s[:start] + replacement + s[end:]
}

func insertAt(s string, offset int, insert string) string {
	if offset < 0 {
		offset = 0
	}
	if offset > len(s) {
		offset = len(s)
	}
	return s[:offset] + insert + s[offset:]
}

func byteToPosition(code string, byteOffset int) masking.Position {
	if byteOffset < 0 {
		byteOffset = 0
	}
	if byteOffset > len(code) {
		byteOffset = len(code)
	}

	line := 0
	char := 0
	for _, r := range code[:byteOffset] {
		if r == '\n' {
			line++
			char = 0
		} else {
			char++
		}
	}
	return masking.Position{Line: line, Character: char}
}

func generateMaskID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "mask_" + uuid.New().String()
	}
	return "mask_" + hex.EncodeToString(bytes)
}
