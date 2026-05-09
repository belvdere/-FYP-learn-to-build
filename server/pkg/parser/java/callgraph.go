package java

import (
	"context"
	"fmt"
	"os"
	"strings"

	"example.com/fyp/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
	javats "github.com/smacker/go-tree-sitter/java"
)

// JavaCallGraphExtractor implements CallGraphExtractor for Java.
type JavaCallGraphExtractor struct{}

// NewJavaCallGraphExtractor creates a new Java call graph extractor.
func NewJavaCallGraphExtractor() *JavaCallGraphExtractor {
	return &JavaCallGraphExtractor{}
}

// CallGraphExtractor extracts method call relationships from Java AST.
type CallGraphExtractor struct {
	filePath      string
	sourceCode    []byte
	currentMethod string            // Tracks the current method context
	currentClass  string            // Tracks the current class context
	calls         []types.MethodCall
	seen          map[string]bool   // Deduplication: "caller:callee:line" -> bool
	fieldTypes    map[string]string // fieldName -> typeName (for resolving object.method calls)
	localTypes    map[string]string // localVar -> typeName (for current method scope)
}

// ExtractMethodCalls extracts all method invocations from a Java file.
func (e *JavaCallGraphExtractor) ExtractMethodCalls(filePath string, sourceCode []byte) ([]types.MethodCall, error) {
	p := sitter.NewParser()
	lang := javats.GetLanguage()
	if lang == nil {
		return nil, fmt.Errorf("failed to get Java language")
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

	extractor := &CallGraphExtractor{
		filePath:   filePath,
		sourceCode: sourceCode,
		calls:      make([]types.MethodCall, 0),
		seen:       make(map[string]bool),
		fieldTypes: make(map[string]string),
		localTypes: make(map[string]string),
	}

	// First pass: extract field types
	extractor.extractFieldTypes(tree.RootNode())

	// Second pass: extract method calls
	extractor.walkNode(tree.RootNode())

	fmt.Fprintf(os.Stderr, "[CallGraph] Extracted %d method calls from %s\n", len(extractor.calls), filePath)

	return extractor.calls, nil
}

// walkNode recursively walks the AST to extract method calls.
func (e *CallGraphExtractor) walkNode(node *sitter.Node) {
	nodeType := node.Type()

	// Track class context
	if nodeType == "class_declaration" || nodeType == "interface_declaration" {
		className := e.extractClassName(node)
		if className != "" {
			prevClass := e.currentClass
			e.currentClass = className
			e.walkChildren(node)
			e.currentClass = prevClass
			return
		}
	}

	// Track method context
	if nodeType == "method_declaration" || nodeType == "constructor_declaration" {
		methodName := e.extractMethodName(node)
		if methodName != "" {
			prevMethod := e.currentMethod
			// Qualified method name: ClassName.methodName or ClassName.<init>
			if e.currentClass != "" {
				if nodeType == "constructor_declaration" {
					e.currentMethod = e.currentClass + ".<init>"
				} else {
					e.currentMethod = e.currentClass + "." + methodName
				}
			} else {
				e.currentMethod = methodName
			}
			// Extract local variable types for this method
			e.extractLocalVariableTypes(node)
			e.walkChildren(node)
			e.currentMethod = prevMethod
			return
		}
	}

	// Extract method invocations
	if nodeType == "method_invocation" {
		e.extractMethodInvocation(node)
		// Still walk children to catch nested/chained calls (deduplication handles duplicates)
		e.walkChildren(node)
		return
	}

	// Extract constructor invocations (new Foo(...))
	if nodeType == "object_creation_expression" {
		e.extractConstructorInvocation(node)
		// Walk children to catch method calls in constructor arguments
		e.walkChildren(node)
		return
	}

	// Extract method references (e.g., ClassName::method, this::validate)
	if nodeType == "method_reference" {
		e.extractMethodReference(node)
		return
	}

	// Extract constructor chaining: this(...) and super(...)
	if nodeType == "explicit_constructor_invocation" {
		e.extractExplicitConstructorInvocation(node)
		e.walkChildren(node)
		return
	}

	// Lambda expressions: preserve caller context when descending into body
	if nodeType == "lambda_expression" {
		e.walkChildren(node)
		return
	}

	// Continue walking
	e.walkChildren(node)
}

// walkChildren walks all child nodes.
func (e *CallGraphExtractor) walkChildren(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			e.walkNode(child)
		}
	}
}

// extractClassName extracts the class name from a class_declaration node.
func (e *CallGraphExtractor) extractClassName(node *sitter.Node) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "identifier" {
			return child.Content(e.sourceCode)
		}
	}
	return ""
}

// extractMethodName extracts the method name from a method_declaration node.
func (e *CallGraphExtractor) extractMethodName(node *sitter.Node) string {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type() == "identifier" {
			return child.Content(e.sourceCode)
		}
	}
	return ""
}

// extractMethodInvocation extracts a method call from a method_invocation node.
func (e *CallGraphExtractor) extractMethodInvocation(node *sitter.Node) {
	if e.currentMethod == "" {
		// Skip calls outside of method context (e.g., field initializers)
		return
	}

	// Extract the called method name and object/class
	calleeInfo := e.extractCalleeInfo(node)
	if calleeInfo == "" {
		return
	}

	// Determine call type
	callType := e.determineCallType(node)

	// Create unique key for deduplication
	line := int(node.StartPoint().Row) + 1
	key := fmt.Sprintf("%s:%s:%d", e.currentMethod, calleeInfo, line)

	// Check if we've already seen this exact call
	if e.seen[key] {
		return
	}
	e.seen[key] = true

	// Create method call record
	call := types.MethodCall{
		CallerID: e.currentMethod,
		CalleeID: calleeInfo,
		CallType:     callType,
		FilePath:     e.filePath,
		Line:         line,
	}

	e.calls = append(e.calls, call)
}

// extractConstructorInvocation extracts a constructor call from an object_creation_expression node.
// e.g., "new User(email)" -> CallerID: "UserService.createUser", CalleeID: "User.<init>"
func (e *CallGraphExtractor) extractConstructorInvocation(node *sitter.Node) {
	if e.currentMethod == "" {
		// Skip constructor calls outside of method context
		return
	}

	// Find the type_identifier child to get the class name
	var className string
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		if childType == "type_identifier" {
			className = child.Content(e.sourceCode)
			break
		}
		// Handle generic types like "new ArrayList<String>()"
		if childType == "generic_type" {
			// Get the type_identifier within the generic_type
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild != nil && grandchild.Type() == "type_identifier" {
					className = grandchild.Content(e.sourceCode)
					break
				}
			}
			break
		}
	}

	if className == "" {
		return
	}

	calleeSymbol := className + ".<init>"

	// Create unique key for deduplication
	line := int(node.StartPoint().Row) + 1
	key := fmt.Sprintf("%s:%s:%d", e.currentMethod, calleeSymbol, line)

	if e.seen[key] {
		return
	}
	e.seen[key] = true

	call := types.MethodCall{
		CallerID: e.currentMethod,
		CalleeID: calleeSymbol,
		CallType:     "constructor",
		FilePath:     e.filePath,
		Line:         line,
	}

	e.calls = append(e.calls, call)
}

// extractCalleeInfo extracts the called method name from a method_invocation node.
// Returns fully qualified names like: "ClassName.methodName"
// Resolves variable types: "mongoTemplate.count" -> "MongoTemplate.count"
func (e *CallGraphExtractor) extractCalleeInfo(node *sitter.Node) string {
	var objectPart string
	var methodPart string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		childType := child.Type()

		// Handle field_access nodes (e.g., System.out in System.out.println)
		if childType == "field_access" {
			nextSibling := node.Child(i + 1)
			if nextSibling != nil && nextSibling.Type() == "." {
				// Extract the full field access path
				objectPart = child.Content(e.sourceCode)
			}
		}
		
		// Extract object/class part (before the dot)
		if childType == "identifier" {
			// Check if this is the object part (comes before method name)
			nextSibling := node.Child(i + 1)
			if nextSibling != nil && nextSibling.Type() == "." {
				objectPart = child.Content(e.sourceCode)
			}
		}
		
		// Handle chained method calls: obj.method1().method2()
		// The object part is a method_invocation node
		if childType == "method_invocation" {
			nextSibling := node.Child(i + 1)
			if nextSibling != nil && nextSibling.Type() == "." {
				// For chained calls, we can't easily resolve the return type
				// So we'll skip this edge and only track the inner call
				// This is acceptable as the inner call will be tracked separately
				return ""
			}
		}

		// Extract method name (after the dot or standalone)
		if childType == "identifier" {
			// Check if this is the method name (comes after dot or is standalone)
			prevSibling := node.Child(i - 1)
			if prevSibling != nil && prevSibling.Type() == "." {
				methodPart = child.Content(e.sourceCode)
			} else if objectPart == "" && !e.hasMethodInvocationSibling(node, i) && !e.hasFieldAccessSibling(node, i) {
				// Standalone method call (no object, not part of chain)
				methodPart = child.Content(e.sourceCode)
			}
		}
	}

	// Construct the callee info
	if methodPart == "" {
		return ""
	}

	if objectPart != "" {
		// Check if it's a field access (e.g., System.out)
		if strings.Contains(objectPart, ".") {
			// For field access, keep the last part only for type resolution
			// e.g., "System.out" -> just use "System" as the type
			parts := strings.Split(objectPart, ".")
			if len(parts) > 0 {
				firstPart := parts[0]
				// If first part starts with uppercase, it's likely a class (keep full path)
				if len(firstPart) > 0 && firstPart[0] >= 'A' && firstPart[0] <= 'Z' {
					// Keep the full qualified name for external libraries
					return objectPart + "." + methodPart
				}
			}
		}
		
		// Resolve the object type
		// e.g., "mongoTemplate" -> "MongoTemplate"
		resolvedType := e.resolveType(objectPart)
		return resolvedType + "." + methodPart
	}

	// Standalone method call - assume it's in the current class
	if e.currentClass != "" {
		return e.currentClass + "." + methodPart
	}

	return methodPart
}

// hasMethodInvocationSibling checks if the node has a method_invocation sibling before it.
// This helps identify if we're in a chained call situation.
func (e *CallGraphExtractor) hasMethodInvocationSibling(parent *sitter.Node, currentIdx int) bool {
	for i := 0; i < currentIdx; i++ {
		sibling := parent.Child(i)
		if sibling != nil && sibling.Type() == "method_invocation" {
			return true
		}
	}
	return false
}

// hasFieldAccessSibling checks if the node has a field_access sibling before it.
func (e *CallGraphExtractor) hasFieldAccessSibling(parent *sitter.Node, currentIdx int) bool {
	for i := 0; i < currentIdx; i++ {
		sibling := parent.Child(i)
		if sibling != nil && sibling.Type() == "field_access" {
			return true
		}
	}
	return false
}

// determineCallType determines the type of method call.
func (e *CallGraphExtractor) determineCallType(node *sitter.Node) string {
	// Check for static calls (ClassName.method)
	content := node.Content(e.sourceCode)

	// Simple heuristics:
	// - If starts with uppercase, likely static (ClassName.method)
	// - If has "super.", it's a super call
	// - Otherwise, it's a direct call

	if strings.HasPrefix(content, "super.") {
		return "super"
	}

	// Check if first character is uppercase (likely static)
	if len(content) > 0 && content[0] >= 'A' && content[0] <= 'Z' {
		return "static"
	}

	return "direct"
}

// extractFieldTypes does a first pass to extract field types from the class.
func (e *CallGraphExtractor) extractFieldTypes(node *sitter.Node) {
	if node == nil {
		return
	}

	nodeType := node.Type()

	// Track class context
	if nodeType == "class_declaration" || nodeType == "interface_declaration" {
		className := e.extractClassName(node)
		if className != "" {
			prevClass := e.currentClass
			e.currentClass = className
			e.extractFieldTypesInChildren(node)
			e.currentClass = prevClass
			return
		}
	}

	// Extract field types
	if nodeType == "field_declaration" {
		e.extractFieldTypeInfo(node)
	}

	// Continue walking
	e.extractFieldTypesInChildren(node)
}

// extractFieldTypesInChildren walks all child nodes for field extraction.
func (e *CallGraphExtractor) extractFieldTypesInChildren(node *sitter.Node) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			e.extractFieldTypes(child)
		}
	}
}

// extractFieldTypeInfo extracts the type and name from a field_declaration node.
// Example: "private MongoTemplate mongoTemplate;" -> fieldTypes["mongoTemplate"] = "MongoTemplate"
func (e *CallGraphExtractor) extractFieldTypeInfo(node *sitter.Node) {
	var typeName string
	var fieldName string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		childType := child.Type()

		// Extract type (e.g., "MongoTemplate", "String", "List<String>")
		if childType == "type_identifier" || childType == "generic_type" || childType == "integral_type" {
			typeName = child.Content(e.sourceCode)
			// For generic types, strip the type parameters (List<String> -> List)
			if idx := strings.Index(typeName, "<"); idx >= 0 {
				typeName = typeName[:idx]
			}
		}

		// Extract field name from variable_declarator
		if childType == "variable_declarator" {
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild != nil && grandchild.Type() == "identifier" {
					fieldName = grandchild.Content(e.sourceCode)
					break
				}
			}
		}
	}

	// Store the mapping
	if typeName != "" && fieldName != "" {
		e.fieldTypes[fieldName] = typeName
	}
}

// extractLocalVariableTypes extracts local variable types from method body.
func (e *CallGraphExtractor) extractLocalVariableTypes(node *sitter.Node) {
	// Clear local types for new method scope
	e.localTypes = make(map[string]string)
	e.walkNodeForLocalVariables(node)
}

// walkNodeForLocalVariables walks the AST to find local variable declarations.
func (e *CallGraphExtractor) walkNodeForLocalVariables(node *sitter.Node) {
	if node == nil {
		return
	}

	if node.Type() == "local_variable_declaration" {
		e.extractLocalVariableTypeInfo(node)
	}

	// Continue walking
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil {
			e.walkNodeForLocalVariables(child)
		}
	}
}

// extractLocalVariableTypeInfo extracts type and name from a local_variable_declaration node.
// Example: "MongoTemplate template = ...;" -> localTypes["template"] = "MongoTemplate"
func (e *CallGraphExtractor) extractLocalVariableTypeInfo(node *sitter.Node) {
	var typeName string
	var varName string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}

		childType := child.Type()

		// Extract type
		if childType == "type_identifier" || childType == "generic_type" || childType == "integral_type" {
			typeName = child.Content(e.sourceCode)
			// For generic types, strip the type parameters
			if idx := strings.Index(typeName, "<"); idx >= 0 {
				typeName = typeName[:idx]
			}
		}

		// Extract variable name
		if childType == "variable_declarator" {
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild != nil && grandchild.Type() == "identifier" {
					varName = grandchild.Content(e.sourceCode)
					break
				}
			}
		}
	}

	// Store the mapping
	if typeName != "" && varName != "" {
		e.localTypes[varName] = typeName
	}
}

// extractMethodReference handles method reference expressions: ClassName::method or this::method.
func (e *CallGraphExtractor) extractMethodReference(node *sitter.Node) {
	if e.currentMethod == "" {
		return
	}

	// AST: method_reference has children: [object, "::", identifier]
	// object can be: type_identifier, identifier, "this", "super"
	var objectPart string
	var methodPart string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		childType := child.Type()
		content := child.Content(e.sourceCode)

		if childType == "::" {
			continue
		}
		if childType == "identifier" || childType == "type_identifier" {
			if objectPart == "" {
				objectPart = content
			} else {
				methodPart = content
			}
		} else if content == "this" || content == "super" {
			objectPart = content
		}
	}

	if methodPart == "" {
		return
	}

	// Resolve the object to a type
	var calleeSymbol string
	if objectPart == "this" || objectPart == "" {
		calleeSymbol = e.currentClass + "." + methodPart
	} else if objectPart == "super" {
		calleeSymbol = "super." + methodPart
	} else {
		resolvedType := e.resolveType(objectPart)
		calleeSymbol = resolvedType + "." + methodPart
	}

	line := int(node.StartPoint().Row) + 1
	key := fmt.Sprintf("%s:%s:%d", e.currentMethod, calleeSymbol, line)
	if e.seen[key] {
		return
	}
	e.seen[key] = true

	e.calls = append(e.calls, types.MethodCall{
		CallerID: e.currentMethod,
		CalleeID: calleeSymbol,
		CallType:     "method_reference",
		FilePath:     e.filePath,
		Line:         line,
	})
}

// extractExplicitConstructorInvocation handles this(...) and super(...) constructor chaining.
func (e *CallGraphExtractor) extractExplicitConstructorInvocation(node *sitter.Node) {
	if e.currentMethod == "" {
		return
	}

	// Determine if this is "this" or "super" invocation
	var callType string
	var calleeSymbol string

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		content := child.Content(e.sourceCode)
		if content == "this" {
			callType = "constructor"
			calleeSymbol = e.currentClass + ".<init>"
			break
		} else if content == "super" {
			callType = "super"
			calleeSymbol = "super.<init>"
			break
		}
	}

	if calleeSymbol == "" {
		return
	}

	line := int(node.StartPoint().Row) + 1
	key := fmt.Sprintf("%s:%s:%d", e.currentMethod, calleeSymbol, line)
	if e.seen[key] {
		return
	}
	e.seen[key] = true

	e.calls = append(e.calls, types.MethodCall{
		CallerID: e.currentMethod,
		CalleeID: calleeSymbol,
		CallType:     callType,
		FilePath:     e.filePath,
		Line:         line,
	})
}

// resolveType resolves a variable/field name to its actual type.
// Examples:
//   - "mongoTemplate" -> "MongoTemplate"
//   - "System.out" -> "System" (can't fully resolve, keep as is)
//   - "TextUtils" -> "TextUtils" (already a class name)
func (e *CallGraphExtractor) resolveType(name string) string {
	// Check if it's a local variable
	if typeName, exists := e.localTypes[name]; exists {
		return typeName
	}

	// Check if it's a field
	if typeName, exists := e.fieldTypes[name]; exists {
		return typeName
	}

	// If it starts with uppercase, assume it's already a class name
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		return name
	}

	// Can't resolve, return as is
	return name
}
