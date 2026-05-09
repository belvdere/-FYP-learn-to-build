package java

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
	javaparser "github.com/smacker/go-tree-sitter/java"
)

// JavaMasker implements Masker for Java.
type JavaMasker struct {
	parser *sitter.Parser
	config *masking.MaskingConfig
}

// NewJavaMasker creates a new Java masker instance.
func NewJavaMasker() *JavaMasker {
	parser := sitter.NewParser()
	parser.SetLanguage(javaparser.GetLanguage())
	return &JavaMasker{
		parser: parser,
		config: masking.DefaultMaskingConfig(),
	}
}

// NewJavaMaskerWithConfig creates a Java masker with custom configuration.
func NewJavaMaskerWithConfig(config *masking.MaskingConfig) *JavaMasker {
	parser := sitter.NewParser()
	parser.SetLanguage(javaparser.GetLanguage())
	return &JavaMasker{
		parser: parser,
		config: config,
	}
}

// MaskCode analyzes code and inserts mask markers at decision points.
func (m *JavaMasker) MaskCode(code string) (*masking.MaskResult, error) {
	// Parse code into AST
	ctx := context.Background()
	tree, err := m.parser.ParseCtx(ctx, nil, []byte(code))
	if err != nil {
		return nil, fmt.Errorf("parse code: %w", err)
	}
	defer tree.Close()

	// Check for parse errors in the tree
	root := tree.RootNode()
	if root.HasError() {
		return nil, fmt.Errorf("code contains syntax errors")
	}

	// Identify decision points using deterministic rules
	decisionPoints := m.identifyDecisionPoints(tree, []byte(code))

	// Sort by priority first so the limit keeps the most important decision points.
	sortDecisionPointsByPriority(decisionPoints)

	// Limit to max masks (applied after priority sort, so we keep the top-priority ones).
	if m.config.MaxMasksPerFunction > 0 && len(decisionPoints) > m.config.MaxMasksPerFunction {
		decisionPoints = decisionPoints[:m.config.MaxMasksPerFunction]
	}

	// Insert mask markers - MUST process from end to start (by byte offset)
	// so that earlier positions don't shift when we replace text.
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

// MaskCodeWithStore masks code and stores the session in the database.
// Injects the session ID into each mask comment so the VS Code extension
// can extract it for validation: /* [MASK:session=<uuid> id=<maskId> hint="..."] */
// Returns: masked code, session ID, error
func (m *JavaMasker) MaskCodeWithStore(code string, filePath string, store *db.Store) (string, string, error) {
	// Perform masking
	result, err := m.MaskCode(code)
	if err != nil {
		return "", "", fmt.Errorf("mask code: %w", err)
	}

	// Generate UUID session ID
	sessionID := uuid.New().String()

	// Inject session ID into each mask comment so the extension can find it.
	// Before: /* [MASK:id=mask_abc hint="..."] */
	// After:  /* [MASK:session=<uuid> id=mask_abc hint="..."] */
	maskedCode := strings.ReplaceAll(
		result.MaskedCode,
		"/* [MASK:id=",
		fmt.Sprintf("/* [MASK:session=%s id=", sessionID),
	)

	// Store in database
	session := &db.MaskSession{
		ID:           sessionID,
		FilePath:     filePath,
		OriginalCode: code,
		MaskedCode:   maskedCode,
		Language:     "java",
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
func (m *JavaMasker) identifyDecisionPoints(tree *sitter.Tree, content []byte) []masking.DecisionPoint {
	points := []masking.DecisionPoint{}

	root := tree.RootNode()

	// Traverse AST to find decision points
	m.traverseNode(root, content, &points)

	return points
}

// traverseNode recursively traverses AST nodes.
func (m *JavaMasker) traverseNode(node *sitter.Node, content []byte, points *[]masking.DecisionPoint) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	// Check for maskable patterns
	switch nodeType {
	case "if_statement":
		if m.config.MaskValidation {
			point := analyzeIfStatement(node, content, m.config)
			if point != nil {
				*points = append(*points, *point)
			}
		}
	case "switch_expression", "switch_statement":
		if m.config.MaskValidation {
			point := analyzeSwitchStatement(node, content, m.config)
			if point != nil {
				*points = append(*points, *point)
			}
		}
	case "ternary_expression", "conditional_expression":
		if m.config.MaskValidation {
			point := analyzeTernaryExpression(node, content, m.config)
			if point != nil {
				*points = append(*points, *point)
			}
		}
	case "return_statement":
		if m.config.MaskReturns {
			point := analyzeReturnStatement(node, content, m.config)
			if point != nil {
				*points = append(*points, *point)
			}
		}
	case "throw_statement":
		if m.config.MaskErrors {
			point := analyzeThrowStatement(node, content, m.config)
			if point != nil {
				*points = append(*points, *point)
			}
		}
	}

	// Traverse children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		m.traverseNode(child, content, points)
	}
}

// insertMaskMarker inserts a type-aware placeholder + mask comment at the decision point.
// The placeholder keeps the file syntactically valid while clearly marking what the user must fill in.
func (m *JavaMasker) insertMaskMarker(code string, point masking.DecisionPoint) (masking.Mask, string) {
	maskID := generateMaskID()

	// Create mask marker comment
	maskComment := fmt.Sprintf("/* [MASK:id=%s hint=\"%s\"] */", maskID, point.Hint)

	// Build a type-aware placeholder that keeps the code syntactically valid
	var placeholder string
	switch point.Type {
	case masking.MaskTypeCondition, masking.MaskTypeValidation:
		// For conditions (if/switch/ternary): replace with (true /* [MASK:...] */)
		// The original code is the parenthesized expression like "(x == null || x.isEmpty())"
		placeholder = fmt.Sprintf("(true %s)", maskComment)
	case masking.MaskTypeReturn:
		// For return statements: replace with "return null; /* [MASK:...] */"
		placeholder = fmt.Sprintf("return null; %s", maskComment)
	case masking.MaskTypeError:
		// For throw statements: replace with "throw new RuntimeException(); /* [MASK:...] */"
		placeholder = fmt.Sprintf("throw new RuntimeException(); %s", maskComment)
	default:
		// Fallback: just the marker comment (for any future mask types)
		placeholder = maskComment
	}

	// Replace the original code with the placeholder
	before := code[:point.StartByte]
	after := code[point.EndByte:]
	maskedCode := before + placeholder + after

	// Calculate line/character positions
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
// Iterates over runes (not bytes) so multi-byte UTF-8 characters are counted
// as a single character, giving correct positions for non-ASCII source files.
func (m *JavaMasker) byteToPosition(code string, byteOffset int) masking.Position {
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

// generateMaskID generates a unique mask ID using crypto/rand.
// Falls back to uuid if rand.Read fails (should not happen on supported platforms).
func generateMaskID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "mask_" + uuid.New().String()
	}
	return "mask_" + hex.EncodeToString(bytes)
}

// sortDecisionPointsByPriority sorts decision points by priority (descending)
func sortDecisionPointsByPriority(points []masking.DecisionPoint) {
	sort.Slice(points, func(i, j int) bool {
		return points[j].Priority < points[i].Priority
	})
}

// sortDecisionPointsByByteOffsetDesc sorts by StartByte descending (end of file first).
// This order is required for correct insertion: when we replace text, later positions
// are unaffected, but earlier positions would shift if we inserted there first.
func sortDecisionPointsByByteOffsetDesc(points []masking.DecisionPoint) {
	sort.Slice(points, func(i, j int) bool {
		return points[i].StartByte > points[j].StartByte
	})
}
