package java

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"

	"example.com/fyp/pkg/segmentation"
)

// JavaSegmenter segments Java classes into methods
type JavaSegmenter struct {
	parser *sitter.Parser
}

// NewJavaSegmenter creates a new Java segmenter
func NewJavaSegmenter() *JavaSegmenter {
	parser := sitter.NewParser()
	parser.SetLanguage(java.GetLanguage())
	return &JavaSegmenter{
		parser: parser,
	}
}

// SegmentClass segments a Java class into individual methods
func (s *JavaSegmenter) SegmentClass(code string) ([]segmentation.MethodSegment, error) {
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
	s.extractMethods(root, []byte(code), &methods)

	if len(methods) == 0 {
		return nil, fmt.Errorf("no methods found in code")
	}

	return methods, nil
}

// extractMethods recursively extracts all methods from the AST
func (s *JavaSegmenter) extractMethods(node *sitter.Node, code []byte, methods *[]segmentation.MethodSegment) {
	if node == nil {
		return
	}

	// Check if this is a method declaration
	if node.Type() == "method_declaration" {
		method := s.parseMethod(node, code)
		if method != nil {
			*methods = append(*methods, *method)
		}
	}

	// Recursively search child nodes
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		s.extractMethods(child, code, methods)
	}
}

// parseMethod extracts method information from a method_declaration node
func (s *JavaSegmenter) parseMethod(node *sitter.Node, code []byte) *segmentation.MethodSegment {
	method := &segmentation.MethodSegment{
		StartLine: int(node.StartPoint().Row) + 1, // 1-indexed
		EndLine:   int(node.EndPoint().Row) + 1,   // 1-indexed
	}

	// Extract full method body
	method.Body = string(node.Content(code))

	// Parse modifiers
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)

		switch child.Type() {
		case "modifiers":
			modifiers := string(child.Content(code))
			method.IsStatic = strings.Contains(modifiers, "static")
			method.IsPublic = strings.Contains(modifiers, "public")

		case "type_identifier", "void_type", "integral_type", "floating_point_type", "boolean_type":
			method.ReturnType = string(child.Content(code))

		case "identifier":
			method.Name = string(child.Content(code))

		case "formal_parameters":
			method.Parameters = s.parseParameters(child, code)
		}
	}

	// Build signature
	var sb strings.Builder
	if method.IsPublic {
		sb.WriteString("public ")
	}
	if method.IsStatic {
		sb.WriteString("static ")
	}
	if method.ReturnType != "" {
		sb.WriteString(method.ReturnType)
		sb.WriteString(" ")
	}
	sb.WriteString(method.Name)
	sb.WriteString("(")
	for i, param := range method.Parameters {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(param.Type)
		sb.WriteString(" ")
		sb.WriteString(param.Name)
	}
	sb.WriteString(")")
	method.Signature = sb.String()

	return method
}

// parseParameters extracts parameters from formal_parameters node
func (s *JavaSegmenter) parseParameters(node *sitter.Node, code []byte) []segmentation.Parameter {
	params := []segmentation.Parameter{}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "formal_parameter" {
			param := s.parseParameter(child, code)
			if param != nil {
				params = append(params, *param)
			}
		}
	}

	return params
}

// parseParameter extracts a single parameter
func (s *JavaSegmenter) parseParameter(node *sitter.Node, code []byte) *segmentation.Parameter {
	param := &segmentation.Parameter{}

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)

		switch child.Type() {
		case "type_identifier", "integral_type", "floating_point_type", "boolean_type":
			param.Type = string(child.Content(code))
		case "generic_type":
			param.Type = string(child.Content(code))
		case "identifier":
			param.Name = string(child.Content(code))
		}
	}

	return param
}
