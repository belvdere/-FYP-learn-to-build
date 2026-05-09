package ecmascript

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"example.com/fyp/pkg/parser"
	"example.com/fyp/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
)

// ECMAScriptSymbolExtractor extracts symbols from JS/TS ASTs.
type ECMAScriptSymbolExtractor struct{}

type paramInfo struct {
	name string
	typ  string
}

// NewECMAScriptSymbolExtractor creates a symbol extractor.
func NewECMAScriptSymbolExtractor() *ECMAScriptSymbolExtractor {
	return &ECMAScriptSymbolExtractor{}
}

// ExtractSymbols extracts class/function/method/constructor symbols.
func (e *ECMAScriptSymbolExtractor) ExtractSymbols(tree *sitter.Tree, content []byte, filePath string) []types.Symbol {
	root := tree.RootNode()
	moduleName := moduleNameFromPath(filePath)
	symbols := make([]types.Symbol, 0, 24)

	var walk func(node *sitter.Node, className string, functionDepth int)
	walk = func(node *sitter.Node, className string, functionDepth int) {
		if node == nil {
			return
		}

		switch node.Type() {
		case "class_declaration":
			nameNode := node.ChildByFieldName("name")
			newClassName := className
			if nameNode != nil {
				newClassName = parser.GetNodeText(nameNode, content)
				signature := extractSignature(node, content)
				symbols = append(symbols, types.Symbol{
					ID:        generateSymbolID(filePath, newClassName, "class", signature),
					Name:      newClassName,
					Kind:      "class",
					FilePath:  filePath,
					Line:      int(nameNode.StartPoint().Row) + 1,
					Col:       int(nameNode.StartPoint().Column),
					EndLine:   int(node.EndPoint().Row) + 1,
					Signature: signature,
				})
			}
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), newClassName, functionDepth)
			}
			return

		case "method_definition":
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), className, functionDepth+1)
			}
			if functionDepth > 0 || className == "" {
				return
			}
			if sym := extractMethodDefinitionSymbol(node, content, filePath, className); sym != nil {
				symbols = append(symbols, *sym)
			}
			return

		case "function_declaration":
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), className, functionDepth+1)
			}
			if functionDepth > 0 {
				return
			}
			if sym := extractFunctionDeclarationSymbol(node, content, filePath, moduleName); sym != nil {
				symbols = append(symbols, *sym)
			}
			return

		case "variable_declarator":
			if functionDepth > 0 {
				return
			}
			if sym := extractVariableFunctionSymbol(node, content, filePath, moduleName); sym != nil {
				symbols = append(symbols, *sym)
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i), className, functionDepth)
		}
	}

	walk(root, "", 0)
	return symbols
}

func extractMethodDefinitionSymbol(node *sitter.Node, content []byte, filePath, className string) *types.Symbol {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		nameNode = firstChildByType(node, "property_identifier", "private_property_identifier", "identifier")
	}
	if nameNode == nil {
		return nil
	}

	rawName := parser.GetNodeText(nameNode, content)
	if rawName == "" {
		return nil
	}

	signature := extractSignature(node, content)
	kind := "method"
	qualified := className + "." + rawName
	if rawName == "constructor" {
		kind = "constructor"
		qualified = className + ".<init>"
	}

	paramTypes := extractParameterTypes(node.ChildByFieldName("parameters"), content)
	qualifiedWithParams := qualified + "(" + strings.Join(paramTypes, ",") + ")"

	return &types.Symbol{
		ID:        generateSymbolID(filePath, qualifiedWithParams, kind, signature),
		Name:      qualifiedWithParams,
		Kind:      kind,
		FilePath:  filePath,
		Line:      int(nameNode.StartPoint().Row) + 1,
		Col:       int(nameNode.StartPoint().Column),
		EndLine:   int(node.EndPoint().Row) + 1,
		Signature: signature,
		BodyHash:  extractCallableBodyHash(node, content),
	}
}

func extractFunctionDeclarationSymbol(node *sitter.Node, content []byte, filePath, moduleName string) *types.Symbol {
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

	signature := extractSignature(node, content)
	paramTypes := extractParameterTypes(node.ChildByFieldName("parameters"), content)
	qualifiedWithParams := moduleName + "." + fnName + "(" + strings.Join(paramTypes, ",") + ")"

	return &types.Symbol{
		ID:        generateSymbolID(filePath, qualifiedWithParams, "method", signature),
		Name:      qualifiedWithParams,
		Kind:      "method",
		FilePath:  filePath,
		Line:      int(nameNode.StartPoint().Row) + 1,
		Col:       int(nameNode.StartPoint().Column),
		EndLine:   int(node.EndPoint().Row) + 1,
		Signature: signature,
		BodyHash:  extractCallableBodyHash(node, content),
	}
}

func extractVariableFunctionSymbol(node *sitter.Node, content []byte, filePath, moduleName string) *types.Symbol {
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

	fnName := parser.GetNodeText(nameNode, content)
	if fnName == "" {
		return nil
	}

	signature := extractSignature(node, content)
	paramTypes := extractParameterTypes(valueNode.ChildByFieldName("parameters"), content)
	qualifiedWithParams := moduleName + "." + fnName + "(" + strings.Join(paramTypes, ",") + ")"

	return &types.Symbol{
		ID:        generateSymbolID(filePath, qualifiedWithParams, "method", signature),
		Name:      qualifiedWithParams,
		Kind:      "method",
		FilePath:  filePath,
		Line:      int(nameNode.StartPoint().Row) + 1,
		Col:       int(nameNode.StartPoint().Column),
		EndLine:   int(valueNode.EndPoint().Row) + 1,
		Signature: signature,
		BodyHash:  extractCallableBodyHash(valueNode, content),
	}
}

func moduleNameFromPath(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if name == "" {
		return "module"
	}
	return name
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

func extractSignature(node *sitter.Node, content []byte) string {
	text := parser.GetNodeText(node, content)
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return ""
	}
	sig := strings.TrimSpace(lines[0])
	if len(sig) > 240 {
		sig = sig[:240] + "..."
	}
	return sig
}

func generateSymbolID(filePath, name, kind, signature string) string {
	data := fmt.Sprintf("%s:%s:%s:%s", filePath, name, kind, signature)
	h := sha256.Sum256([]byte(data))
	return "sym_" + hex.EncodeToString(h[:16])
}

func extractParameterTypes(paramsNode *sitter.Node, content []byte) []string {
	if paramsNode == nil {
		return nil
	}

	paramText := strings.TrimSpace(parser.GetNodeText(paramsNode, content))
	if paramText == "" {
		return nil
	}
	if strings.HasPrefix(paramText, "(") && strings.HasSuffix(paramText, ")") {
		paramText = strings.TrimSpace(paramText[1 : len(paramText)-1])
	}
	if paramText == "" {
		return nil
	}

	tokens := splitParams(paramText)
	infos := make([]paramInfo, 0, len(tokens))
	for _, token := range tokens {
		if info, ok := parseParamToken(token); ok {
			infos = append(infos, info)
		}
	}

	out := make([]string, 0, len(infos))
	for _, info := range infos {
		out = append(out, info.typ)
	}
	return out
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

func parseParamToken(token string) (paramInfo, bool) {
	t := strings.TrimSpace(token)
	if t == "" {
		return paramInfo{}, false
	}
	t = strings.TrimPrefix(t, "...")
	t = strings.TrimPrefix(t, "readonly ")
	if idx := strings.Index(t, "="); idx >= 0 {
		t = strings.TrimSpace(t[:idx])
	}
	if t == "" {
		return paramInfo{}, false
	}

	name := t
	typ := "_"
	if idx := strings.Index(t, ":"); idx >= 0 {
		name = strings.TrimSpace(t[:idx])
		ann := strings.TrimSpace(t[idx+1:])
		if ann != "" {
			typ = normalizeTypeText(ann)
		}
	}
	name = strings.TrimSuffix(name, "?")
	name = strings.TrimSpace(name)
	if name == "" {
		name = "_"
	}
	return paramInfo{name: name, typ: typ}, true
}

func normalizeTypeText(s string) string {
	if s == "" {
		return "_"
	}
	return strings.Join(strings.Fields(s), "")
}

func extractCallableBodyHash(node *sitter.Node, content []byte) string {
	bodyNode := node.ChildByFieldName("body")
	if bodyNode == nil {
		bodyNode = node
	}

	tokens := make([]string, 0, 48)
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
