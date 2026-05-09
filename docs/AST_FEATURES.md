# How AST Powers Our Features

This document explains how **tree-sitter's Abstract Syntax Tree (AST)** enables masking, validation, symbol extraction, and segmentation across the learn-to-build backend.

## AST at a Glance

We use **tree-sitter** (`github.com/smacker/go-tree-sitter`) with language grammars for **Java, Python, TypeScript, and JavaScript** to parse source code into an AST. Each node has:

- **Type** – e.g. `if_statement`, `method_declaration`, `method_invocation`
- **Byte range** – `StartByte()`, `EndByte()` for extracting text from source
- **Position** – `StartPoint()`, `EndPoint()` (line/column)
- **Hierarchy** – `ChildCount()`, `Child(i)`, `Parent()`
- **Named fields** – `ChildByFieldName("condition")` for grammar-defined relationships

---

## 1. Masking

**Location:** `server/pkg/masking/java/masker.go`, `decision_points.go`

**Goal:** Replace critical decision points (conditionals, returns, error handling) with mask placeholders so developers must fill them in themselves.

### AST Features Used

| Feature | How It's Used |
|---------|---------------|
| **Node type dispatch** | `traverseNode()` switches on `node.Type()` to detect `if_statement`, `return_statement`, `throw_statement`, `switch_statement`, `ternary_expression` |
| **Named fields** | `node.ChildByFieldName("condition")` to get the condition inside an `if_statement` |
| **Byte ranges** | `StartByte()`/`EndByte()` to slice original source for replacement and for `OriginalCode` |
| **Tree traversal** | Depth-first recursion over `Child(i)` to find all decision points |
| **Position info** | `StartPoint()`/`EndPoint()` for `Range` in mask metadata (line/column for editor) |

### Why AST Instead of Regex

- **Structural accuracy** – Knows the exact condition in `if (x == null) return;`, not just "first `if`"
- **No false positives** – Avoids masking strings/comments that look like code
- **Consistency** – Same logic for nested `if`, ternary, switch
- **Type-aware placeholders** – Can insert valid Java (`return null;`, `throw new RuntimeException();`) at the right spans

### Key Node Types for Masking

```
if_statement          → mask condition (ChildByFieldName("condition"))
switch_statement      → mask expression being switched on
ternary_expression    → mask condition (first child before ?)
return_statement      → mask entire return (or condition)
throw_statement       → mask entire throw
```

---

## 2. Validation

**Location:** `server/pkg/validation/java/validator.go`

**Goal:** Ensure filled code is syntactically valid before further processing.

### AST Features Used

| Feature | How It's Used |
|---------|---------------|
| **Parse success** | If `parser.ParseCtx()` fails or returns nil, code is invalid |
| **HasError()** | `root.HasError()` on the tree reports parse recovery/errors |
| **ERROR nodes** | `findFirstErrorNode()` walks AST looking for `node.Type() == "ERROR"` |
| **Position info** | `StartPoint()` on the error node for line/column in feedback |

### Why AST Instead of Regex

- **Deterministic** – Tree-sitter produces a real parse; invalid Java fails immediately
- **Error location** – Can point to the exact node that failed
- **Error recovery** – Tree-sitter often produces a partial tree even with errors, so we can still locate issues

### Flow

```
code → Parse → tree
       ↓
       root.HasError()? → YES → findFirstErrorNode() → line/col for user
       ↓ NO
       Pass
```

---

## 3. Call Graph (LSP-Resolved)

**Location:** Extension indexing bridge (`build-with-me/src/features/indexing/lspBridge.ts`)

**Goal:** Build compiler-precise caller/callee relationships.

Workspace callgraph extraction is no longer built from heuristic AST traversal. The flow is:

1. Go scanner (`index.scan`) uses tree-sitter to emit callable symbols + source positions.
2. Extension asks language servers (`vscode.prepareCallHierarchy`, `vscode.provideOutgoingCalls`) for resolved outgoing calls.
3. Go API stores canonical edges via `index.storeEdges` / `index.appendEdges`.

AST still contributes to callgraph indirectly by providing symbol IDs, signatures, and location anchors for LSP resolution.

---

## 4. Symbols

**Location:** `server/pkg/parser/*/symbols.go`

**Goal:** Extract callable symbols and metadata (ID, name, kind, line/col, signature, body hash) for indexing and graph visualization.

### AST Features Used

| Feature | How It's Used |
|---------|---------------|
| **FindNodesByType()** | `FindNodesByType(root, "class_declaration")`, `method_declaration`, `field_declaration`, etc. |
| **FindChildByType()** | Get `identifier` child for name inside a declaration |
| **GetParentOfType()** | Resolve `method_declaration` → `class_declaration` for qualified names (`UserService.save`) |
| **GetNodeText()** | Slice source by byte range for signature/snippet |
| **Position** | `StartPoint().Row + 1` for 1-indexed line in Symbol |

### Why AST Instead of Regex

- **Declaration vs usage** – Only declarations create symbols; usages are distinct in the AST
- **Scoping** – Parent lookup gives correct qualified names
- **Nested structures** – Fields inside classes, methods inside classes/interfaces
- **Signature extraction** – Full declaration span for display

### Key Node Types for Symbols

```
class_declaration      → class symbol (identifier = name)
interface_declaration  → interface symbol
method_declaration     → method symbol (qualified with class)
constructor_declaration → constructor symbol (ClassName.<init>)
field_declaration      → field symbols (variable_declarator per field)
```

---

## 5. Segmentation

**Location:** `server/pkg/segmentation/java/segmenter.go`

**Goal:** Split a class into individual method segments for LLM prompting or per-method masking.

### AST Features Used

| Feature | How It's Used |
|---------|---------------|
| **Node type filter** | Recursively find `method_declaration` nodes |
| **Byte range** | `node.Content(code)` for full method body text |
| **Position** | `StartPoint().Row`, `EndPoint().Row` for StartLine/EndLine |
| **Child iteration** | Walk children for `modifiers`, `type_identifier`, `identifier`, `formal_parameters` |
| **Structured extraction** | `formal_parameters` → `formal_parameter` → type + name |

### Why AST Instead of Regex

- **Method boundaries** – Uses real parse boundaries, not brace counting
- **Nested classes** – Correctly segments inner class methods
- **Parameters** – `formal_parameters` gives structured param list (type, name)
- **Modifiers** – `modifiers` node for `static`, `public`, etc.

### Key Node Types for Segmentation

```
method_declaration   → one MethodSegment per node
  modifiers          → IsStatic, IsPublic
  type_identifier / void_type → ReturnType
  identifier         → Name
  formal_parameters  → Parameters (formal_parameter → Type, Name)
  block              → Body (full text via Content)
```

---

## Shared Parser Helpers

**Location:** `server/pkg/parser/interface.go`

These helpers are used across symbols, call graph, and other features:

| Helper | AST Features Used | Purpose |
|--------|-------------------|---------|
| `GetNodeText(node, content)` | `StartByte()`, `EndByte()` | Slice source for node |
| `FindNodesByType(root, type)` | `Type()`, `ChildCount()`, `Child(i)` | Collect all nodes of a type |
| `GetParentOfType(node, type)` | `Parent()` | Find enclosing class/interface |
| `FindChildByType(node, type)` | `ChildCount()`, `Child(i)`, `Type()` | Get first matching child |

---

## Trying It: ast-dump

To see the AST for any Java snippet:

```bash
# From stdin
echo 'public class Foo { void bar() { if (x == null) return; } }' | go run ./cmd/ast-dump

# With --code
go run ./cmd/ast-dump --code 'public class Foo { void bar() {} }'

# From file
go run ./cmd/ast-dump < src/main/java/Example.java
```

Output format: `NodeType [Lstart:col–Lend:col] "text preview"` in a tree layout.

---

## Summary

| Feature | Primary AST Use | Key Node Types |
|---------|-----------------|----------------|
| **Masking** | Traverse to find decision points, use byte ranges for replacement | if_statement, return_statement, throw_statement, switch, ternary |
| **Validation** | Parse success + HasError + ERROR nodes | (root), ERROR |
| **Call Graph** | LSP call hierarchy resolution (AST provides symbol/position inputs) | N/A in extraction phase |
| **Symbols** | Find declarations, parent for qualified names, emit stable IDs | class/function declarations, method_declaration |
| **Segmentation** | Find method_declaration, extract body and metadata | method_declaration, formal_parameters, modifiers |

Masking, validation, symbol extraction, and segmentation rely on tree-sitter’s precise structure rather than regex or string matching.
