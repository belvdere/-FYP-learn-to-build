package golang

import (
	"context"
	"fmt"
	"os"
	"strings"

	"example.com/fyp/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	golangts "github.com/smacker/go-tree-sitter/golang"
)

// GoCallGraphExtractor implements CallGraphExtractor for Go.
type GoCallGraphExtractor struct{}

// NewGoCallGraphExtractor creates a new Go call graph extractor.
func NewGoCallGraphExtractor() *GoCallGraphExtractor {
	return &GoCallGraphExtractor{}
}

// goCallExtractor holds state during AST traversal.
type goCallExtractor struct {
	filePath    string
	sourceCode  []byte
	currentFunc string // e.g. "FuncName" or "ReceiverType.MethodName"
	calls       []types.MethodCall
	seen        map[string]bool
}

// ExtractMethodCalls extracts all call relationships from a Go file.
func (e *GoCallGraphExtractor) ExtractMethodCalls(filePath string, sourceCode []byte) ([]types.MethodCall, error) {
	p := sitter.NewParser()
	lang := golangts.GetLanguage()
	if lang == nil {
		return nil, fmt.Errorf("failed to get Go language")
	}
	p.SetLanguage(lang)

	tree, err := p.ParseCtx(context.Background(), nil, sourceCode)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}
	if tree == nil {
		return nil, fmt.Errorf("parse tree is nil")
	}
	defer tree.Close()

	ext := &goCallExtractor{
		filePath:   filePath,
		sourceCode: sourceCode,
		calls:      make([]types.MethodCall, 0),
		seen:       make(map[string]bool),
	}

	ext.walkNode(tree.RootNode())

	fmt.Fprintf(os.Stderr, "[CallGraph] Extracted %d method calls from %s\n", len(ext.calls), filePath)

	return ext.calls, nil
}

func (e *goCallExtractor) walkNode(node *sitter.Node) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	switch nodeType {
	case "function_declaration":
		funcName := e.extractFuncName(node)
		if funcName != "" {
			prev := e.currentFunc
			e.currentFunc = funcName
			e.walkChildren(node)
			e.currentFunc = prev
			return
		}

	case "method_declaration":
		methodName := e.extractMethodName(node)
		if methodName != "" {
			prev := e.currentFunc
			e.currentFunc = methodName
			e.walkChildren(node)
			e.currentFunc = prev
			return
		}

	case "call_expression":
		e.extractCallExpression(node)
		e.walkChildren(node)
		return
	}

	e.walkChildren(node)
}

func (e *goCallExtractor) walkChildren(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			e.walkNode(child)
		}
	}
}

// extractFuncName returns the name of a function_declaration node.
func (e *goCallExtractor) extractFuncName(node *sitter.Node) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "identifier" {
			return child.Content(e.sourceCode)
		}
	}
	return ""
}

// extractMethodName returns "ReceiverType.MethodName" for a method_declaration node.
func (e *goCallExtractor) extractMethodName(node *sitter.Node) string {
	receiverType := extractReceiverTypeName(node, e.sourceCode)

	// method name is in a field_identifier child
	var methodName string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type() == "field_identifier" {
			methodName = child.Content(e.sourceCode)
			break
		}
		// fallback: identifier that is not in the receiver
		if child.Type() == "identifier" && receiverType != "" {
			methodName = child.Content(e.sourceCode)
		}
	}

	if methodName == "" {
		return ""
	}
	if receiverType != "" {
		return receiverType + "." + methodName
	}
	return methodName
}

// extractReceiverTypeName extracts the receiver type (stripping *) from a method_declaration.
func extractReceiverTypeName(node *sitter.Node, src []byte) string {
	receiverNode := node.ChildByFieldName("receiver")
	if receiverNode == nil {
		return ""
	}
	for i := 0; i < int(receiverNode.ChildCount()); i++ {
		child := receiverNode.Child(i)
		if child == nil || child.Type() != "parameter_declaration" {
			continue
		}
		for j := 0; j < int(child.ChildCount()); j++ {
			tc := child.Child(j)
			if tc == nil {
				continue
			}
			switch tc.Type() {
			case "type_identifier":
				return tc.Content(src)
			case "pointer_type":
				for k := 0; k < int(tc.ChildCount()); k++ {
					inner := tc.Child(k)
					if inner != nil && inner.Type() == "type_identifier" {
						return inner.Content(src)
					}
				}
			}
		}
	}
	return ""
}

// extractCallExpression records a call_expression node.
func (e *goCallExtractor) extractCallExpression(node *sitter.Node) {
	if e.currentFunc == "" {
		return
	}

	// call_expression has a "function" field
	funcNode := node.ChildByFieldName("function")
	if funcNode == nil {
		return
	}

	callee, callType := e.resolveCallee(funcNode)
	if callee == "" {
		return
	}

	line := int(node.StartPoint().Row) + 1
	key := fmt.Sprintf("%s:%s:%d", e.currentFunc, callee, line)
	if e.seen[key] {
		return
	}
	e.seen[key] = true

	e.calls = append(e.calls, types.MethodCall{
		CallerID: e.currentFunc,
		CalleeID: callee,
		CallType: callType,
		FilePath: e.filePath,
		Line:     line,
	})
}

// resolveCallee converts the function node of a call_expression into a callee string.
func (e *goCallExtractor) resolveCallee(funcNode *sitter.Node) (callee string, callType string) {
	switch funcNode.Type() {
	case "identifier":
		// Bare call: foo()
		return funcNode.Content(e.sourceCode), "direct"

	case "selector_expression":
		// obj.Method() or pkg.Func()
		operand := funcNode.ChildByFieldName("operand")
		field := funcNode.ChildByFieldName("field")
		if operand == nil || field == nil {
			return "", ""
		}
		operandName := strings.TrimSpace(operand.Content(e.sourceCode))
		fieldName := strings.TrimSpace(field.Content(e.sourceCode))
		callee = operandName + "." + fieldName

		// Heuristic: starts with uppercase → package-level (static), else method call
		if len(operandName) > 0 && operandName[0] >= 'A' && operandName[0] <= 'Z' {
			callType = "static"
		} else {
			callType = "direct"
		}
		return callee, callType
	}

	return "", ""
}
