package python

import (
	"context"
	"fmt"
	"strings"

	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/segmentation"
	sitter "github.com/smacker/go-tree-sitter"
	pythonlang "github.com/smacker/go-tree-sitter/python"
)

// PythonSegmenter segments Python code into callable units.
type PythonSegmenter struct {
	parser *sitter.Parser
}

// NewPythonSegmenter creates a new Python segmenter.
func NewPythonSegmenter() *PythonSegmenter {
	p := sitter.NewParser()
	p.SetLanguage(pythonlang.GetLanguage())
	return &PythonSegmenter{parser: p}
}

// SegmentClass segments Python source into top-level functions and class methods.
func (s *PythonSegmenter) SegmentClass(code string) ([]segmentation.MethodSegment, error) {
	tree, err := s.parser.ParseCtx(context.Background(), nil, []byte(code))
	if err != nil {
		return nil, fmt.Errorf("failed to parse code: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		return nil, fmt.Errorf("code contains syntax errors")
	}

	segments := make([]segmentation.MethodSegment, 0, 8)
	var walk func(node *sitter.Node, className string, functionDepth int)
	walk = func(node *sitter.Node, className string, functionDepth int) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "decorated_definition":
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), className, functionDepth)
			}
			return

		case "class_definition":
			nameNode := node.ChildByFieldName("name")
			newClass := className
			if nameNode != nil {
				newClass = parser.GetNodeText(nameNode, []byte(code))
			}
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), newClass, functionDepth)
			}
			return

		case "function_definition", "async_function_definition":
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), className, functionDepth+1)
			}
			if functionDepth > 0 {
				return
			}

			if seg := buildSegment(node, []byte(code), className); seg != nil {
				segments = append(segments, *seg)
			}
			return
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

func buildSegment(node *sitter.Node, content []byte, className string) *segmentation.MethodSegment {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	fnName := parser.GetNodeText(nameNode, content)
	paramsNode := node.ChildByFieldName("parameters")
	params := parseParameters(paramsNode, content)

	qualified := fnName
	if className != "" {
		qualified = className + "." + fnName
	}

	signature := strings.TrimSpace(parser.GetNodeText(node, content))
	if idx := strings.Index(signature, "\n"); idx >= 0 {
		signature = strings.TrimSpace(signature[:idx])
	}

	retType := ""
	if rt := node.ChildByFieldName("return_type"); rt != nil {
		retType = strings.TrimSpace(parser.GetNodeText(rt, content))
	}

	return &segmentation.MethodSegment{
		Name:       qualified,
		Signature:  signature,
		Body:       parser.GetNodeText(node, content),
		StartLine:  int(node.StartPoint().Row) + 1,
		EndLine:    int(node.EndPoint().Row) + 1,
		IsStatic:   false,
		IsPublic:   true,
		ReturnType: retType,
		Parameters: params,
	}
}

func parseParameters(paramsNode *sitter.Node, content []byte) []segmentation.Parameter {
	if paramsNode == nil {
		return nil
	}

	text := strings.TrimSpace(parser.GetNodeText(paramsNode, content))
	if len(text) < 2 || text[0] != '(' || text[len(text)-1] != ')' {
		return nil
	}

	parts := splitParams(strings.TrimSpace(text[1 : len(text)-1]))
	out := make([]segmentation.Parameter, 0, len(parts))
	for _, p := range parts {
		name, typ, ok := parseParam(p)
		if !ok {
			continue
		}
		out = append(out, segmentation.Parameter{Name: name, Type: typ})
	}
	return out
}

func splitParams(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := make([]string, 0, 8)
	start := 0
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
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

func parseParam(token string) (name, typ string, ok bool) {
	t := strings.TrimSpace(token)
	if t == "" || t == "/" || t == "*" {
		return "", "", false
	}

	for strings.HasPrefix(t, "*") {
		t = strings.TrimPrefix(t, "*")
	}
	if idx := strings.Index(t, "="); idx >= 0 {
		t = strings.TrimSpace(t[:idx])
	}
	if t == "" {
		return "", "", false
	}

	typ = ""
	name = t
	if idx := strings.Index(t, ":"); idx >= 0 {
		name = strings.TrimSpace(t[:idx])
		typ = strings.TrimSpace(t[idx+1:])
	}
	if name == "" {
		return "", "", false
	}
	return name, typ, true
}
