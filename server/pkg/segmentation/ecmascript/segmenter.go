package ecmascript

import (
	"context"
	"fmt"
	"strings"

	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/segmentation"
	sitter "github.com/smacker/go-tree-sitter"
	jslang "github.com/smacker/go-tree-sitter/javascript"
	tsxlang "github.com/smacker/go-tree-sitter/typescript/tsx"
)

// ECMAScriptSegmenter segments JS/TS code into callable units.
type ECMAScriptSegmenter struct {
	parser *sitter.Parser
}

// NewJavaScriptSegmenter creates a segmenter for JavaScript.
func NewJavaScriptSegmenter() *ECMAScriptSegmenter {
	p := sitter.NewParser()
	p.SetLanguage(jslang.GetLanguage())
	return &ECMAScriptSegmenter{parser: p}
}

// NewTypeScriptSegmenter creates a segmenter for TypeScript/TSX.
func NewTypeScriptSegmenter() *ECMAScriptSegmenter {
	p := sitter.NewParser()
	p.SetLanguage(tsxlang.GetLanguage())
	return &ECMAScriptSegmenter{parser: p}
}

// SegmentClass segments source into top-level functions and class methods.
func (s *ECMAScriptSegmenter) SegmentClass(code string) ([]segmentation.MethodSegment, error) {
	tree, err := s.parser.ParseCtx(context.Background(), nil, []byte(code))
	if err != nil {
		return nil, fmt.Errorf("failed to parse code: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		return nil, fmt.Errorf("code contains syntax errors")
	}

	segments := make([]segmentation.MethodSegment, 0, 12)

	var walk func(node *sitter.Node, className string, functionDepth int)
	walk = func(node *sitter.Node, className string, functionDepth int) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "class_declaration":
			nameNode := node.ChildByFieldName("name")
			nextClass := className
			if nameNode != nil {
				nextClass = parser.GetNodeText(nameNode, []byte(code))
			}
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), nextClass, functionDepth)
			}
			return

		case "method_definition":
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), className, functionDepth+1)
			}
			if functionDepth > 0 || className == "" {
				return
			}
			if seg := buildMethodSegment(node, []byte(code), className); seg != nil {
				segments = append(segments, *seg)
			}
			return

		case "function_declaration":
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), className, functionDepth+1)
			}
			if functionDepth > 0 {
				return
			}
			if seg := buildFunctionSegment(node, []byte(code)); seg != nil {
				segments = append(segments, *seg)
			}
			return

		case "variable_declarator":
			if functionDepth > 0 {
				return
			}
			if seg := buildVariableFunctionSegment(node, []byte(code)); seg != nil {
				segments = append(segments, *seg)
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i), className, functionDepth)
		}
	}

	walk(root, "", 0)
	if len(segments) == 0 {
		return nil, fmt.Errorf("no methods found in code")
	}
	return segments, nil
}

func buildMethodSegment(node *sitter.Node, content []byte, className string) *segmentation.MethodSegment {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		nameNode = firstChildByType(node, "property_identifier", "private_property_identifier", "identifier")
	}
	if nameNode == nil {
		return nil
	}

	name := parser.GetNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	params, paramTypes := parseParameters(node.ChildByFieldName("parameters"), content)
	qualified := className + "." + name
	if name == "constructor" {
		qualified = className + ".<init>"
	}

	signature := strings.TrimSpace(parser.GetNodeText(node, content))
	if idx := strings.Index(signature, "\n"); idx >= 0 {
		signature = strings.TrimSpace(signature[:idx])
	}
	if signature == "" {
		signature = qualified + "(" + strings.Join(paramTypes, ",") + ")"
	}

	return &segmentation.MethodSegment{
		Name:       qualified,
		Signature:  signature,
		Body:       parser.GetNodeText(node, content),
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(node.EndPoint().Row) + 1,
		IsStatic:   false,
		IsPublic:   true,
		ReturnType: "",
		Parameters: params,
	}
}

func buildFunctionSegment(node *sitter.Node, content []byte) *segmentation.MethodSegment {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		nameNode = firstChildByType(node, "identifier")
	}
	if nameNode == nil {
		return nil
	}

	fnName := parser.GetNodeText(nameNode, content)
	if fnName == "" {
		return nil
	}

	params, paramTypes := parseParameters(node.ChildByFieldName("parameters"), content)
	signature := strings.TrimSpace(parser.GetNodeText(node, content))
	if idx := strings.Index(signature, "\n"); idx >= 0 {
		signature = strings.TrimSpace(signature[:idx])
	}
	if signature == "" {
		signature = fnName + "(" + strings.Join(paramTypes, ",") + ")"
	}

	return &segmentation.MethodSegment{
		Name:       fnName,
		Signature:  signature,
		Body:       parser.GetNodeText(node, content),
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(node.EndPoint().Row) + 1,
		IsStatic:   false,
		IsPublic:   true,
		ReturnType: "",
		Parameters: params,
	}
}

func buildVariableFunctionSegment(node *sitter.Node, content []byte) *segmentation.MethodSegment {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		nameNode = firstChildByType(node, "identifier")
	}
	if nameNode == nil {
		return nil
	}

	valueNode := node.ChildByFieldName("value")
	if valueNode == nil {
		return nil
	}
	switch valueNode.Type() {
	case "arrow_function", "function", "function_expression":
	default:
		return nil
	}

	name := parser.GetNodeText(nameNode, content)
	if name == "" {
		return nil
	}

	params, paramTypes := parseParameters(valueNode.ChildByFieldName("parameters"), content)
	signature := strings.TrimSpace(parser.GetNodeText(node, content))
	if idx := strings.Index(signature, "\n"); idx >= 0 {
		signature = strings.TrimSpace(signature[:idx])
	}
	if signature == "" {
		signature = name + "(" + strings.Join(paramTypes, ",") + ")"
	}

	return &segmentation.MethodSegment{
		Name:       name,
		Signature:  signature,
		Body:       parser.GetNodeText(node, content),
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(valueNode.EndPoint().Row) + 1,
		IsStatic:   false,
		IsPublic:   true,
		ReturnType: "",
		Parameters: params,
	}
}

func parseParameters(paramsNode *sitter.Node, content []byte) ([]segmentation.Parameter, []string) {
	if paramsNode == nil {
		return nil, nil
	}

	paramText := strings.TrimSpace(parser.GetNodeText(paramsNode, content))
	if paramText == "" {
		return nil, nil
	}
	if strings.HasPrefix(paramText, "(") && strings.HasSuffix(paramText, ")") {
		paramText = strings.TrimSpace(paramText[1 : len(paramText)-1])
	}
	if paramText == "" {
		return nil, nil
	}

	tokens := splitParams(paramText)
	params := make([]segmentation.Parameter, 0, len(tokens))
	paramTypes := make([]string, 0, len(tokens))
	for _, token := range tokens {
		name, typ, ok := parseParamToken(token)
		if !ok {
			continue
		}
		params = append(params, segmentation.Parameter{Name: name, Type: typ})
		paramTypes = append(paramTypes, typ)
	}
	return params, paramTypes
}

func splitParams(s string) []string {
	parts := make([]string, 0, 8)
	start := 0
	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	angleDepth := 0

	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case ',':
			if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && angleDepth == 0 {
				part := strings.TrimSpace(s[start:i])
				if part != "" {
					parts = append(parts, part)
				}
				start = i + 1
			}
		}
	}
	last := strings.TrimSpace(s[start:])
	if last != "" {
		parts = append(parts, last)
	}
	return parts
}

func parseParamToken(token string) (name, typ string, ok bool) {
	t := strings.TrimSpace(token)
	if t == "" {
		return "", "", false
	}
	t = strings.TrimPrefix(t, "...")
	t = strings.TrimPrefix(t, "readonly ")
	if idx := strings.Index(t, "="); idx >= 0 {
		t = strings.TrimSpace(t[:idx])
	}
	if t == "" {
		return "", "", false
	}

	name = t
	typ = "_"
	if idx := strings.Index(t, ":"); idx >= 0 {
		name = strings.TrimSpace(t[:idx])
		ann := strings.TrimSpace(t[idx+1:])
		if ann != "" {
			typ = strings.Join(strings.Fields(ann), "")
		}
	}
	name = strings.TrimSuffix(name, "?")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "_"
	}
	return name, typ, true
}

func firstChildByType(node *sitter.Node, nodeTypes ...string) *sitter.Node {
	typeSet := make(map[string]struct{}, len(nodeTypes))
	for _, t := range nodeTypes {
		typeSet[t] = struct{}{}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if _, ok := typeSet[child.Type()]; ok {
			return child
		}
	}
	return nil
}
