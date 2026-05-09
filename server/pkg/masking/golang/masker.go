package golang

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
	goparser "github.com/smacker/go-tree-sitter/golang"
)

// GoMasker implements Masker for Go.
type GoMasker struct {
	parser *sitter.Parser
	config *masking.MaskingConfig
}

// NewGoMasker creates a new Go masker instance.
func NewGoMasker() *GoMasker {
	parser := sitter.NewParser()
	parser.SetLanguage(goparser.GetLanguage())
	return &GoMasker{
		parser: parser,
		config: masking.DefaultMaskingConfig(),
	}
}

// NewGoMaskerWithConfig creates a Go masker with custom configuration.
func NewGoMaskerWithConfig(config *masking.MaskingConfig) *GoMasker {
	parser := sitter.NewParser()
	parser.SetLanguage(goparser.GetLanguage())
	return &GoMasker{
		parser: parser,
		config: config,
	}
}

// MaskCode analyzes Go code and inserts mask markers at decision points.
func (m *GoMasker) MaskCode(code string) (*masking.MaskResult, error) {
	ctx := context.Background()
	tree, err := m.parser.ParseCtx(ctx, nil, []byte(code))
	if err != nil {
		return nil, fmt.Errorf("parse code: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		return nil, fmt.Errorf("code contains syntax errors")
	}

	decisionPoints := m.identifyDecisionPoints(tree, []byte(code))

	sortDecisionPointsByPriority(decisionPoints)

	if m.config.MaxMasksPerFunction > 0 && len(decisionPoints) > m.config.MaxMasksPerFunction {
		decisionPoints = decisionPoints[:m.config.MaxMasksPerFunction]
	}

	sortDecisionPointsByByteOffsetDesc(decisionPoints)

	maskedCode := code
	masks := []masking.Mask{}

	for _, point := range decisionPoints {
		mask, newCode := m.insertMaskMarker(maskedCode, point)
		maskedCode = newCode
		masks = append(masks, mask)
	}

	return &masking.MaskResult{
		MaskedCode: maskedCode,
		Masks:      masks,
	}, nil
}

// MaskCodeWithStore masks Go code and stores the session in the database.
func (m *GoMasker) MaskCodeWithStore(code string, filePath string, store *db.Store) (string, string, error) {
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
		Language:     "golang",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		Status:       db.StatusPending,
	}

	if err := store.CreateMaskSession(session); err != nil {
		return "", "", fmt.Errorf("store session: %w", err)
	}

	return maskedCode, sessionID, nil
}

// identifyDecisionPoints finds all maskable decision points in the AST.
func (m *GoMasker) identifyDecisionPoints(tree *sitter.Tree, content []byte) []masking.DecisionPoint {
	points := []masking.DecisionPoint{}
	m.traverseNode(tree.RootNode(), content, &points)
	return points
}

// traverseNode recursively traverses AST nodes looking for decision points.
func (m *GoMasker) traverseNode(node *sitter.Node, content []byte, points *[]masking.DecisionPoint) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch nodeType {
	case "if_statement":
		if m.config.MaskValidation {
			point := analyzeGoIfStatement(node, content, m.config)
			if point != nil {
				*points = append(*points, *point)
			}
		}
	case "expression_switch_statement":
		if m.config.MaskValidation {
			point := analyzeGoSwitchStatement(node, content, m.config)
			if point != nil {
				*points = append(*points, *point)
			}
		}
	case "return_statement":
		if m.config.MaskReturns {
			point := analyzeGoReturnStatement(node, content, m.config)
			if point != nil {
				*points = append(*points, *point)
			}
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		m.traverseNode(child, content, points)
	}
}

// insertMaskMarker inserts a type-aware placeholder + mask comment at the decision point.
func (m *GoMasker) insertMaskMarker(code string, point masking.DecisionPoint) (masking.Mask, string) {
	maskID := generateGoMaskID()
	maskComment := fmt.Sprintf("/* [MASK:id=%s hint=\"%s\"] */", maskID, point.Hint)

	var placeholder string
	switch point.Type {
	case masking.MaskTypeCondition, masking.MaskTypeValidation:
		placeholder = fmt.Sprintf("(true %s)", maskComment)
	case masking.MaskTypeReturn:
		placeholder = fmt.Sprintf("return nil %s", maskComment)
	default:
		placeholder = maskComment
	}

	before := code[:point.StartByte]
	after := code[point.EndByte:]
	maskedCode := before + placeholder + after

	startPos := m.byteToPosition(code, int(point.StartByte))
	endPos := m.byteToPosition(code, int(point.EndByte))

	mask := masking.Mask{
		ID:           maskID,
		Type:         point.Type,
		Hint:         point.Hint,
		OriginalCode: point.OriginalCode,
		Range: masking.Range{
			Start: startPos,
			End:   endPos,
		},
	}

	return mask, maskedCode
}

// byteToPosition converts byte offset to line/character position.
func (m *GoMasker) byteToPosition(code string, byteOffset int) masking.Position {
	line := 0
	char := 0

	if byteOffset > len(code) {
		byteOffset = len(code)
	}

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

// generateGoMaskID generates a unique mask ID using crypto/rand.
func generateGoMaskID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "mask_" + uuid.New().String()
	}
	return "mask_" + hex.EncodeToString(bytes)
}

func sortDecisionPointsByPriority(points []masking.DecisionPoint) {
	sort.Slice(points, func(i, j int) bool {
		return points[j].Priority < points[i].Priority
	})
}

func sortDecisionPointsByByteOffsetDesc(points []masking.DecisionPoint) {
	sort.Slice(points, func(i, j int) bool {
		return points[i].StartByte > points[j].StartByte
	})
}
