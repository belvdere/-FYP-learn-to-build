package golang

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
)

// GoSymbolExtractor implements SymbolExtractor for Go.
type GoSymbolExtractor struct{}

// NewGoSymbolExtractor creates a new Go symbol extractor.
func NewGoSymbolExtractor() *GoSymbolExtractor {
	return &GoSymbolExtractor{}
}

// ExtractSymbols extracts all symbols from a parsed Go file.
func (e *GoSymbolExtractor) ExtractSymbols(tree *sitter.Tree, content []byte, filePath string) []types.Symbol {
	var symbols []types.Symbol

	root := tree.RootNode()

	// Extract top-level function declarations
	functions := parser.FindNodesByType(root, "function_declaration")
	for _, fn := range functions {
		sym := extractFunctionSymbol(fn, content, filePath)
		if sym != nil {
			symbols = append(symbols, *sym)
		}
	}

	// Extract method declarations
	methods := parser.FindNodesByType(root, "method_declaration")
	for _, m := range methods {
		sym := extractGoMethodSymbol(m, content, filePath)
		if sym != nil {
			symbols = append(symbols, *sym)
		}
	}

	// Extract type declarations (structs and interfaces)
	typeDecls := parser.FindNodesByType(root, "type_declaration")
	for _, td := range typeDecls {
		syms := extractTypeSymbols(td, content, filePath)
		symbols = append(symbols, syms...)
	}

	return symbols
}

// extractFunctionSymbol extracts a top-level function symbol.
func extractFunctionSymbol(node *sitter.Node, content []byte, filePath string) *types.Symbol {
	nameNode := parser.FindChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := parser.GetNodeText(nameNode, content)
	paramTypes := extractGoParamTypes(node, content)
	qualifiedWithParams := name + "(" + strings.Join(paramTypes, ",") + ")"
	signature := extractGoSignature(node, content)
	bodyHash := extractGoBodyHash(node, content)

	return &types.Symbol{
		ID:        generateGoSymbolID(filePath, qualifiedWithParams, "function", signature),
		Name:      qualifiedWithParams,
		Kind:      "function",
		FilePath:  filePath,
		Line:      int(nameNode.StartPoint().Row) + 1,
		Col:       int(nameNode.StartPoint().Column),
		EndLine:   int(node.EndPoint().Row) + 1,
		Signature: signature,
		BodyHash:  bodyHash,
	}
}

// extractGoMethodSymbol extracts a method symbol (receiver.MethodName).
func extractGoMethodSymbol(node *sitter.Node, content []byte, filePath string) *types.Symbol {
	nameNode := parser.FindChildByType(node, "field_identifier")
	if nameNode == nil {
		nameNode = parser.FindChildByType(node, "identifier")
	}
	if nameNode == nil {
		return nil
	}

	methodName := parser.GetNodeText(nameNode, content)
	receiverType := extractReceiverType(node, content)

	var qualifiedName string
	if receiverType != "" {
		qualifiedName = receiverType + "." + methodName
	} else {
		qualifiedName = methodName
	}

	paramTypes := extractGoParamTypes(node, content)
	qualifiedWithParams := qualifiedName + "(" + strings.Join(paramTypes, ",") + ")"
	signature := extractGoSignature(node, content)
	bodyHash := extractGoBodyHash(node, content)

	return &types.Symbol{
		ID:        generateGoSymbolID(filePath, qualifiedWithParams, "method", signature),
		Name:      qualifiedWithParams,
		Kind:      "method",
		FilePath:  filePath,
		Line:      int(nameNode.StartPoint().Row) + 1,
		Col:       int(nameNode.StartPoint().Column),
		EndLine:   int(node.EndPoint().Row) + 1,
		Signature: signature,
		BodyHash:  bodyHash,
	}
}

// extractReceiverType extracts the receiver type from a method declaration.
// Strips leading '*' from pointer receivers.
func extractReceiverType(node *sitter.Node, content []byte) string {
	receiverNode := node.ChildByFieldName("receiver")
	if receiverNode == nil {
		return ""
	}

	// parameter_list -> parameter_declaration -> type child
	for i := 0; i < int(receiverNode.ChildCount()); i++ {
		child := receiverNode.Child(i)
		if child == nil || child.Type() != "parameter_declaration" {
			continue
		}
		// Find the type node within the parameter_declaration
		for j := 0; j < int(child.ChildCount()); j++ {
			typeChild := child.Child(j)
			if typeChild == nil {
				continue
			}
			switch typeChild.Type() {
			case "type_identifier":
				return parser.GetNodeText(typeChild, content)
			case "pointer_type":
				// *TypeName — get the type_identifier underneath
				for k := 0; k < int(typeChild.ChildCount()); k++ {
					inner := typeChild.Child(k)
					if inner != nil && inner.Type() == "type_identifier" {
						return parser.GetNodeText(inner, content)
					}
				}
			}
		}
	}
	return ""
}

// extractTypeSymbols extracts struct and interface symbols from a type_declaration.
func extractTypeSymbols(node *sitter.Node, content []byte, filePath string) []types.Symbol {
	var symbols []types.Symbol

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil || child.Type() != "type_spec" {
			continue
		}

		nameNode := parser.FindChildByType(child, "type_identifier")
		if nameNode == nil {
			continue
		}
		name := parser.GetNodeText(nameNode, content)
		signature := extractGoSignature(child, content)

		// Determine kind based on the type body
		kind := ""
		for j := 0; j < int(child.ChildCount()); j++ {
			bodyChild := child.Child(j)
			if bodyChild == nil {
				continue
			}
			switch bodyChild.Type() {
			case "struct_type":
				kind = "struct"
			case "interface_type":
				kind = "interface"
			}
		}

		if kind == "" {
			continue
		}

		symbols = append(symbols, types.Symbol{
			ID:        generateGoSymbolID(filePath, name, kind, signature),
			Name:      name,
			Kind:      kind,
			FilePath:  filePath,
			Line:      int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Signature: signature,
		})
	}

	return symbols
}

// extractGoParamTypes extracts parameter type strings from a function/method node.
func extractGoParamTypes(node *sitter.Node, content []byte) []string {
	// Go uses "parameters" as the field name for function params
	paramsNode := node.ChildByFieldName("parameters")
	if paramsNode == nil {
		paramsNode = parser.FindChildByType(node, "parameter_list")
	}
	if paramsNode == nil {
		return nil
	}

	var types []string
	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil || child.Type() != "parameter_declaration" {
			continue
		}
		typeText := extractGoParamTypeText(child, content)
		if typeText != "" {
			// A single parameter_declaration can declare multiple names: a, b int
			// Count identifiers to repeat the type
			count := countParamNames(child, content)
			for k := 0; k < count; k++ {
				types = append(types, typeText)
			}
		}
	}
	return types
}

func countParamNames(node *sitter.Node, content []byte) int {
	count := 0
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "identifier" {
			count++
		}
	}
	if count == 0 {
		return 1 // anonymous param (type only)
	}
	return count
}

func extractGoParamTypeText(paramNode *sitter.Node, content []byte) string {
	for i := 0; i < int(paramNode.ChildCount()); i++ {
		child := paramNode.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "type_identifier", "pointer_type", "slice_type", "map_type",
			"qualified_type", "array_type", "interface_type", "struct_type",
			"channel_type", "func_type", "variadic_type":
			return strings.TrimSpace(parser.GetNodeText(child, content))
		}
	}
	return ""
}

// extractGoSignature returns the first line of a node (signature line).
func extractGoSignature(node *sitter.Node, content []byte) string {
	text := parser.GetNodeText(node, content)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 {
		sig := strings.TrimSpace(lines[0])
		if len(sig) > 200 {
			sig = sig[:200] + "..."
		}
		return sig
	}
	return ""
}

// generateGoSymbolID generates a unique ID for a Go symbol.
func generateGoSymbolID(filePath, name, kind, signature string) string {
	data := fmt.Sprintf("%s:%s:%s:%s", filePath, name, kind, signature)
	hash := sha256.Sum256([]byte(data))
	return "sym_" + hex.EncodeToString(hash[:16])
}

// extractGoBodyHash computes a SHA256 hash of the function body (block node).
func extractGoBodyHash(node *sitter.Node, content []byte) string {
	bodyNode := parser.FindChildByType(node, "block")
	if bodyNode == nil {
		empty := sha256.Sum256([]byte(""))
		return hex.EncodeToString(empty[:])
	}

	var tokens []string
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		if n == nil {
			return
		}
		if strings.Contains(n.Type(), "comment") {
			return
		}
		if n.ChildCount() == 0 {
			text := strings.TrimSpace(parser.GetNodeText(n, content))
			if text != "" {
				tokens = append(tokens, text)
			}
			return
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(bodyNode)

	normalized := strings.Join(tokens, " ")
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
