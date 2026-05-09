package golang

import (
	"context"
	"fmt"
	"strings"

	"example.com/fyp/pkg/segmentation"
	sitter "github.com/smacker/go-tree-sitter"
	goparser "github.com/smacker/go-tree-sitter/golang"
)

// GoSegmenter segments Go files into functions and methods.
type GoSegmenter struct {
	parser *sitter.Parser
}

// NewGoSegmenter creates a new Go segmenter.
func NewGoSegmenter() *GoSegmenter {
	parser := sitter.NewParser()
	parser.SetLanguage(goparser.GetLanguage())
	return &GoSegmenter{parser: parser}
}

// SegmentClass segments a Go file into individual function/method segments.
func (s *GoSegmenter) SegmentClass(code string) ([]segmentation.MethodSegment, error) {
	ctx := context.Background()
	tree, err := s.parser.ParseCtx(ctx, nil, []byte(code))
	if err != nil {
		return nil, fmt.Errorf("failed to parse code: %w", err)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		return nil, fmt.Errorf("code contains syntax errors")
	}

	methods := []segmentation.MethodSegment{}
	s.extractFunctions(root, []byte(code), &methods)

	if len(methods) == 0 {
		return nil, fmt.Errorf("no functions found in code")
	}

	return methods, nil
}

func (s *GoSegmenter) extractFunctions(node *sitter.Node, code []byte, methods *[]segmentation.MethodSegment) {
	if node == nil {
		return
	}

	switch node.Type() {
	case "function_declaration":
		m := s.parseFuncDecl(node, code, false)
		if m != nil {
			*methods = append(*methods, *m)
		}
	case "method_declaration":
		m := s.parseFuncDecl(node, code, true)
		if m != nil {
			*methods = append(*methods, *m)
		}
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		s.extractFunctions(node.Child(i), code, methods)
	}
}

func (s *GoSegmenter) parseFuncDecl(node *sitter.Node, code []byte, isMethod bool) *segmentation.MethodSegment {
	method := &segmentation.MethodSegment{
		StartLine: int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
		Body:      string(node.Content(code)),
		IsStatic:  !isMethod, // top-level functions are "static" in this context
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "identifier":
			if method.Name == "" {
				method.Name = string(child.Content(code))
				// Public if name starts with uppercase
				if len(method.Name) > 0 && method.Name[0] >= 'A' && method.Name[0] <= 'Z' {
					method.IsPublic = true
				}
			}
		case "field_identifier":
			// Method name in a method_declaration
			method.Name = string(child.Content(code))
			if len(method.Name) > 0 && method.Name[0] >= 'A' && method.Name[0] <= 'Z' {
				method.IsPublic = true
			}
		case "parameter_list":
			// Could be params or result params — check position
			// Parameters come before result; result comes after
			if method.Name != "" && len(method.Parameters) == 0 {
				method.Parameters = s.parseGoParameters(child, code)
			}
		case "type_identifier", "pointer_type", "slice_type", "map_type", "qualified_type":
			method.ReturnType = strings.TrimSpace(string(child.Content(code)))
		}
	}

	// Build signature: func (receiver) Name(params) returnType
	var sb strings.Builder
	sb.WriteString("func ")
	if method.Name != "" {
		sb.WriteString(method.Name)
	}
	sb.WriteString("(")
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		if param.Name != "" {
			sb.WriteString(param.Name)
			sb.WriteString(" ")
		}
		sb.WriteString(param.Type)
	}
	sb.WriteString(")")
	if method.ReturnType != "" {
		sb.WriteString(" ")
		sb.WriteString(method.ReturnType)
	}
	method.Signature = sb.String()

	return method
}

func (s *GoSegmenter) parseGoParameters(node *sitter.Node, code []byte) []segmentation.Parameter {
	params := []segmentation.Parameter{}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || child.Type() != "parameter_declaration" {
			continue
		}
		extracted := s.parseGoParameter(child, code)
		params = append(params, extracted...)
	}
	return params
}

func (s *GoSegmenter) parseGoParameter(node *sitter.Node, code []byte) []segmentation.Parameter {
	var names []string
	var typeName string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "identifier":
			names = append(names, string(child.Content(code)))
		case "type_identifier", "pointer_type", "slice_type", "map_type",
			"qualified_type", "array_type", "interface_type", "variadic_type":
			typeName = strings.TrimSpace(string(child.Content(code)))
		}
	}

	if typeName == "" {
		return nil
	}

	if len(names) == 0 {
		// Anonymous parameter
		return []segmentation.Parameter{{Type: typeName}}
	}

	out := make([]segmentation.Parameter, len(names))
	for i, n := range names {
		out[i] = segmentation.Parameter{Name: n, Type: typeName}
	}
	return out
}
