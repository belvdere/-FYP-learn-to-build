package python

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

// PythonSymbolExtractor implements SymbolExtractor for Python.
type PythonSymbolExtractor struct{}

type paramInfo struct {
	name string
	typ  string
}

// NewPythonSymbolExtractor creates a new Python symbol extractor.
func NewPythonSymbolExtractor() *PythonSymbolExtractor {
	return &PythonSymbolExtractor{}
}

// ExtractSymbols extracts class and callable symbols from a parsed Python file.
func (e *PythonSymbolExtractor) ExtractSymbols(tree *sitter.Tree, content []byte, filePath string) []types.Symbol {
	root := tree.RootNode()
	moduleName := moduleNameFromPath(filePath)
	symbols := make([]types.Symbol, 0, 16)

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
			if nameNode == nil {
				nameNode = firstChildByType(node, "identifier")
			}
			newClassName := className
			if nameNode != nil {
				newClassName = parser.GetNodeText(nameNode, content)
				symbols = append(symbols, types.Symbol{
					ID:        generateSymbolID(filePath, newClassName, "class", extractSignature(node, content)),
					Name:      newClassName,
					Kind:      "class",
					FilePath:  filePath,
					Line:      int(nameNode.StartPoint().Row) + 1,
					Col:       int(nameNode.StartPoint().Column),
					EndLine:   int(node.EndPoint().Row) + 1,
					Signature: extractSignature(node, content),
				})
			}
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), newClassName, functionDepth)
			}
			return

		case "function_definition", "async_function_definition":
			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i), className, functionDepth+1)
			}
			if functionDepth > 0 {
				return
			}

			nameNode := node.ChildByFieldName("name")
			if nameNode == nil {
				nameNode = firstChildByType(node, "identifier")
			}
			if nameNode == nil {
				return
			}

			fnName := parser.GetNodeText(nameNode, content)
			paramsNode := node.ChildByFieldName("parameters")
			paramTypes := extractParameterTypes(paramsNode, content, className != "")
			signature := extractSignature(node, content)
			kind := "method"

			var qualifiedName string
			if className != "" {
				if fnName == "__init__" {
					kind = "constructor"
					qualifiedName = className + ".<init>"
				} else {
					qualifiedName = className + "." + fnName
				}
			} else {
				qualifiedName = moduleName + "." + fnName
			}

			qualifiedWithParams := qualifiedName + "(" + strings.Join(paramTypes, ",") + ")"
			symbols = append(symbols, types.Symbol{
				ID:        generateSymbolID(filePath, qualifiedWithParams, kind, signature),
				Name:      qualifiedWithParams,
				Kind:      kind,
				FilePath:  filePath,
				Line:      int(nameNode.StartPoint().Row) + 1,
				Col:       int(nameNode.StartPoint().Column),
				EndLine:   int(node.EndPoint().Row) + 1,
				Signature: signature,
				BodyHash:  extractCallableBodyHash(node, content),
			})
			return
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i), className, functionDepth)
		}
	}

	walk(root, "", 0)
	return symbols
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

func firstChildByType(node *sitter.Node, nodeType string) *sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == nodeType {
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

func extractParameterTypes(paramsNode *sitter.Node, content []byte, isClassMember bool) []string {
	if paramsNode == nil {
		return nil
	}

	paramText := strings.TrimSpace(parser.GetNodeText(paramsNode, content))
	if len(paramText) < 2 || paramText[0] != '(' || paramText[len(paramText)-1] != ')' {
		return nil
	}
	inner := strings.TrimSpace(paramText[1 : len(paramText)-1])
	if inner == "" {
		return nil
	}

	tokens := splitParams(inner)
	infos := make([]paramInfo, 0, len(tokens))
	for _, token := range tokens {
		if info, ok := parseParamToken(token); ok {
			infos = append(infos, info)
		}
	}

	if isClassMember && len(infos) > 0 {
		first := infos[0].name
		if first == "self" || first == "cls" {
			infos = infos[1:]
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

func parseParamToken(token string) (paramInfo, bool) {
	t := strings.TrimSpace(token)
	if t == "" || t == "/" || t == "*" {
		return paramInfo{}, false
	}

	for strings.HasPrefix(t, "*") {
		t = strings.TrimPrefix(t, "*")
	}
	if t == "" {
		return paramInfo{}, false
	}

	if idx := strings.Index(t, "="); idx >= 0 {
		t = strings.TrimSpace(t[:idx])
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
	if name == "" {
		return paramInfo{}, false
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
		empty := sha256.Sum256([]byte(""))
		return hex.EncodeToString(empty[:])
	}

	tokens := make([]string, 0, 32)
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
