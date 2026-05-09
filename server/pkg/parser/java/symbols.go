package java

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
)

// JavaSymbolExtractor implements SymbolExtractor for Java.
type JavaSymbolExtractor struct{}

// NewJavaSymbolExtractor creates a new Java symbol extractor.
func NewJavaSymbolExtractor() *JavaSymbolExtractor {
	return &JavaSymbolExtractor{}
}

// ExtractSymbols extracts all symbols from a parsed Java file.
func (e *JavaSymbolExtractor) ExtractSymbols(tree *sitter.Tree, content []byte, filePath string) []types.Symbol {
	var symbols []types.Symbol

	root := tree.RootNode()

	// Extract classes and interfaces
	classes := parser.FindNodesByType(root, "class_declaration")
	for _, classNode := range classes {
		symbol := extractClassSymbol(classNode, content, filePath)
		if symbol != nil {
			symbols = append(symbols, *symbol)
		}
	}

	interfaces := parser.FindNodesByType(root, "interface_declaration")
	for _, interfaceNode := range interfaces {
		symbol := extractInterfaceSymbol(interfaceNode, content, filePath)
		if symbol != nil {
			symbols = append(symbols, *symbol)
		}
	}

	// Extract methods (including constructors)
	methods := parser.FindNodesByType(root, "method_declaration")
	for _, methodNode := range methods {
		symbol := extractMethodSymbol(methodNode, content, filePath)
		if symbol != nil {
			symbols = append(symbols, *symbol)
		}
	}

	constructors := parser.FindNodesByType(root, "constructor_declaration")
	for _, constructorNode := range constructors {
		symbol := extractConstructorSymbol(constructorNode, content, filePath)
		if symbol != nil {
			symbols = append(symbols, *symbol)
		}
	}

	// Extract fields
	fields := parser.FindNodesByType(root, "field_declaration")
	for _, fieldNode := range fields {
		fieldSymbols := extractFieldSymbols(fieldNode, content, filePath)
		symbols = append(symbols, fieldSymbols...)
	}

	return symbols
}

// extractClassSymbol extracts a class symbol.
func extractClassSymbol(node *sitter.Node, content []byte, filePath string) *types.Symbol {
	nameNode := parser.FindChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := parser.GetNodeText(nameNode, content)
	signature := extractSignature(node, content)

	return &types.Symbol{
		ID:        generateSymbolID(filePath, name, "class", signature),
		Name:      name,
		Kind:      "class",
		FilePath:  filePath,
		Line:      int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
		Signature: signature,
	}
}

// extractInterfaceSymbol extracts an interface symbol.
func extractInterfaceSymbol(node *sitter.Node, content []byte, filePath string) *types.Symbol {
	nameNode := parser.FindChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := parser.GetNodeText(nameNode, content)
	signature := extractSignature(node, content)

	return &types.Symbol{
		ID:        generateSymbolID(filePath, name, "interface", signature),
		Name:      name,
		Kind:      "interface",
		FilePath:  filePath,
		Line:      int(node.StartPoint().Row) + 1,
		EndLine:   int(node.EndPoint().Row) + 1,
		Signature: signature,
	}
}

// extractMethodSymbol extracts a method symbol.
func extractMethodSymbol(node *sitter.Node, content []byte, filePath string) *types.Symbol {
	nameNode := parser.FindChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	name := parser.GetNodeText(nameNode, content)
	signature := extractSignature(node, content)

	// Get containing class name for fully qualified name
	classNode := parser.GetParentOfType(node, "class_declaration")
	if classNode == nil {
		classNode = parser.GetParentOfType(node, "interface_declaration")
	}

	var qualifiedName string
	if classNode != nil {
		classNameNode := parser.FindChildByType(classNode, "identifier")
		if classNameNode != nil {
			className := parser.GetNodeText(classNameNode, content)
			qualifiedName = className + "." + name
		}
	}

	if qualifiedName == "" {
		qualifiedName = name
	}
	paramTypes := extractMethodParameterTypes(node, content)
	qualifiedWithParams := qualifiedName + "(" + strings.Join(paramTypes, ",") + ")"
	bodyHash := extractCallableBodyHash(node, content)

	return &types.Symbol{
		ID:        generateSymbolID(filePath, qualifiedWithParams, "method", signature),
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

// extractConstructorSymbol extracts a constructor symbol.
// Uses <init> convention: "UserService.<init>" to match call graph extractor output.
func extractConstructorSymbol(node *sitter.Node, content []byte, filePath string) *types.Symbol {
	nameNode := parser.FindChildByType(node, "identifier")
	if nameNode == nil {
		return nil
	}

	signature := extractSignature(node, content)

	// Get containing class name
	classNode := parser.GetParentOfType(node, "class_declaration")
	var qualifiedName string
	if classNode != nil {
		classNameNode := parser.FindChildByType(classNode, "identifier")
		if classNameNode != nil {
			className := parser.GetNodeText(classNameNode, content)
			qualifiedName = className + ".<init>"
		}
	}

	if qualifiedName == "" {
		name := parser.GetNodeText(nameNode, content)
		qualifiedName = name + ".<init>"
	}
	paramTypes := extractMethodParameterTypes(node, content)
	qualifiedWithParams := qualifiedName + "(" + strings.Join(paramTypes, ",") + ")"
	bodyHash := extractCallableBodyHash(node, content)

	return &types.Symbol{
		ID:        generateSymbolID(filePath, qualifiedWithParams, "constructor", signature),
		Name:      qualifiedWithParams,
		Kind:      "constructor",
		FilePath:  filePath,
		Line:      int(nameNode.StartPoint().Row) + 1,
		Col:       int(nameNode.StartPoint().Column),
		EndLine:   int(node.EndPoint().Row) + 1,
		Signature: signature,
		BodyHash:  bodyHash,
	}
}

// extractFieldSymbols extracts field symbols (a field declaration can declare multiple fields).
func extractFieldSymbols(node *sitter.Node, content []byte, filePath string) []types.Symbol {
	var symbols []types.Symbol

	// Find all variable declarators in this field declaration
	declarators := parser.FindNodesByType(node, "variable_declarator")

	for _, declarator := range declarators {
		nameNode := parser.FindChildByType(declarator, "identifier")
		if nameNode == nil {
			continue
		}

		name := parser.GetNodeText(nameNode, content)

		// Get containing class name
		classNode := parser.GetParentOfType(node, "class_declaration")
		var qualifiedName string
		if classNode != nil {
			classNameNode := parser.FindChildByType(classNode, "identifier")
			if classNameNode != nil {
				className := parser.GetNodeText(classNameNode, content)
				qualifiedName = className + "." + name
			}
		}

		if qualifiedName == "" {
			qualifiedName = name
		}

		signature := extractSignature(node, content)

		symbols = append(symbols, types.Symbol{
			ID:        generateSymbolID(filePath, qualifiedName, "field", signature),
			Name:      qualifiedName,
			Kind:      "field",
			FilePath:  filePath,
			Line:      int(node.StartPoint().Row) + 1,
			EndLine:   int(node.EndPoint().Row) + 1,
			Signature: signature,
		})
	}

	return symbols
}

// extractSignature extracts a simplified signature from a node.
func extractSignature(node *sitter.Node, content []byte) string {
	// Get the first line of the node (usually the declaration line)
	text := parser.GetNodeText(node, content)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 {
		// Remove excessive whitespace
		sig := strings.TrimSpace(lines[0])
		if len(sig) > 200 {
			sig = sig[:200] + "..."
		}
		return sig
	}
	return ""
}

// generateSymbolID generates a unique ID for a symbol.
func generateSymbolID(filePath, name, kind, signature string) string {
	data := fmt.Sprintf("%s:%s:%s:%s", filePath, name, kind, signature)
	hash := sha256.Sum256([]byte(data))
	return "sym_" + hex.EncodeToString(hash[:16])
}

func extractMethodParameterTypes(node *sitter.Node, content []byte) []string {
	paramsNode := parser.FindChildByType(node, "formal_parameters")
	if paramsNode == nil {
		return nil
	}

	typesOut := make([]string, 0, int(paramsNode.ChildCount()))
	for i := 0; i < int(paramsNode.ChildCount()); i++ {
		child := paramsNode.Child(i)
		if child == nil {
			continue
		}
		if child.Type() != "formal_parameter" && child.Type() != "spread_parameter" && child.Type() != "receiver_parameter" {
			continue
		}
		t := extractParameterType(child, content)
		if t == "" {
			continue
		}
		typesOut = append(typesOut, t)
	}
	return typesOut
}

func extractParameterType(paramNode *sitter.Node, content []byte) string {
	var typeNode *sitter.Node
	for i := 0; i < int(paramNode.ChildCount()); i++ {
		child := paramNode.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "generic_type", "array_type", "integral_type", "floating_point_type", "void_type", "type_identifier", "scoped_type_identifier", "boolean_type":
			typeNode = child
		}
		if typeNode != nil {
			break
		}
	}
	if typeNode == nil {
		return ""
	}

	typeText := strings.TrimSpace(parser.GetNodeText(typeNode, content))
	if typeText == "" {
		return ""
	}
	if paramNode.Type() == "spread_parameter" && !strings.HasSuffix(typeText, "...") {
		typeText += "..."
	}
	return normalizeTypeText(typeText)
}

func normalizeTypeText(s string) string {
	if s == "" {
		return ""
	}
	return strings.Join(strings.Fields(s), "")
}

func extractCallableBodyHash(node *sitter.Node, content []byte) string {
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
		nodeType := n.Type()
		if strings.Contains(nodeType, "comment") {
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
