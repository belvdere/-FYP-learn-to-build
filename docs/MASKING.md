# Code Masking System

## Purpose

The masking system addresses the **trust-and-ownership gap** in agentic AI pair programming by constraining AI output and forcing developer engagement with critical code.

| Problem | How Masking Helps |
|---------|-------------------|
| Developers blindly accept AI code | Forces developer to write decision logic |
| Reduced code ownership | Developer authors critical sections |
| Hidden logic issues | Developer must understand to fill masks |

---

## What It Does

The masking system analyzes source code and identifies **decision points** - places where learners should practice making programming decisions. It then:

1. **Identifies decision points** (if conditions, returns, error handling, etc.)
2. **Replaces them with mask markers** (`/* [MASK:session=<uuid> id=<maskId> hint="..."] */`)
3. **Stores the session** in a database with original and masked code
4. **Returns masked code** for the user to fill in

This creates **educational coding exercises** where students practice implementing specific logic patterns rather than writing code from scratch.

Supported language plugins currently include Java, Python, TypeScript, and JavaScript. (Examples below use Java syntax.)

**Example:**

```java
// Original code:
public boolean validate(String input) {
    if (input == null || input.isEmpty()) {
        return false;
    }
    return input.length() > 5;
}

// Masked code (for student) - syntactically valid with type-aware placeholders:
public boolean validate(String input) {
    if (true /* [MASK:session=<uuid> id=mask_abc123 hint="add validation check"] */) {
        return false;
    }
    return input.length() > 5;
}
```

The masked code uses **type-aware placeholders** to keep the file syntactically valid:
- Conditions: `(true /* [MASK:session=<uuid> id=<maskId> hint="..."] */)` -- the `true` placeholder keeps `if` valid
- Returns: `return null; /* [MASK:session=<uuid> id=<maskId> hint="..."] */` -- valid return with placeholder value
- Throws: `throw new RuntimeException(); /* [MASK:session=<uuid> id=<maskId> hint="..."] */` -- valid throw with placeholder

---

## Workflow

### Complete Masking Flow

```
1. Copilot calls kg.maskCode(code, filePath)
   ↓
2. Parse & Analyze
   ↓
   ┌────────────────────────────────────────┐
   │  tree-sitter language parser           │
   │  - Parse code → AST                    │
   │  - Walk AST to find decision nodes     │
   └────────────────────────────────────────┘
   
3. Decision Point Identification
   ↓
   ┌────────────────────────────────────────┐
   │  Deterministic Analysis                │
   │  ─────────────────────────────────     │
   │  For each AST node:                    │
   │    - if_statement → Check complexity   │
   │    - switch_expression → Mask cases    │
   │    - return_statement → Mask if complex│
   │    - throw_statement → Mask error      │
   │    - catch_clause → Check if empty     │
   │    - TODO/FIXME → Always mask          │
   │                                        │
   │  Priority Scoring:                     │
   │    - Complexity × 10 (if conditions)   │
   │    - Complexity × 8 (returns)          │
   │    - Fixed priority (throws, TODOs)    │
   └────────────────────────────────────────┘
   
4. Type-Aware Mask Insertion
   ↓
   ┌────────────────────────────────────────┐
   │  Generate Syntactically-Valid Markers  │
   │  ─────────────────────────────────     │
   │  For each decision point:              │
   │    1. Generate unique ID (mask_<hash>) │
   │    2. Determine mask type & hint       │
   │    3. Insert type-aware placeholder:   │
   │       - Condition → (true /* MASK */)  │
   │       - Return → return null; /* MASK  │
   │       - Throw → throw new Runtime...   │
   │    4. Record original code             │
   └────────────────────────────────────────┘
   
5. Database Storage
   ↓
   ┌────────────────────────────────────────┐
   │  Create Mask Session                   │
   │  ─────────────────────────────────     │
   │  INSERT INTO mask_sessions:            │
   │    - session_id (UUID)                 │
   │    - original_code (before masking)    │
   │    - masked_code (with markers)        │
   │    - file_path, language               │
   │    - status = "pending"                │
   │    - created_at, updated_at            │
   └────────────────────────────────────────┘
   
6. Return masked code to Copilot
   ↓
   Copilot writes masked code to the target file
   User fills in [MASK] placeholders in their normal editor
```

### User Fills Masks

Copilot writes the masked code to the target file. The user then manually edits it in their normal editor, replacing the placeholder values with actual code. The file remains syntactically valid throughout (type-aware placeholders ensure this).

### Validation Flow

Validation can be triggered via Copilot (`kg.validateFilledCode` MCP tool) or the VS Code extension (CodeLens / command palette).

```
1. User says "validate" → Copilot reads file, calls kg.validateFilledCode
   OR user clicks "Validate All Masks" CodeLens in editor
   ↓
2. Retrieve Session
   ↓
   SELECT original_code, masked_code FROM mask_sessions WHERE id = ?
   
3. Parse-Only Validation
   ↓
   - tree-sitter parses filled_code for syntax validity
   - For MCP path: returns original_code for Copilot to do semantic comparison
   
4. Update Database
   ↓
   UPDATE mask_sessions SET:
     filled_code = "...",
     status = "validated" (if pass) or "filled" (if fail)

5. Return feedback
   ↓
   Via Copilot: conversational feedback (hints, not answers)
   Via Extension: VS Code diagnostics (inline squiggles + Problems panel)
```

---

## How It Works

### Deterministic Masking Algorithm

The masker uses **tree-sitter** to parse source code and identify decision points using pattern matching:

#### 1. AST Traversal

```go
func (m *JavaMasker) MaskCode(code string) (*MaskResult, error) {
    // Parse code → AST
    tree, err := m.parser.ParseCtx(ctx, nil, []byte(code))
    
    // Walk AST depth-first
    decisionPoints := []DecisionPoint{}
    walkNode(tree.RootNode(), func(node *sitter.Node) {
        switch node.Type() {
        case "if_statement":
            if dp := analyzeIfStatement(node, code, config); dp != nil {
                decisionPoints = append(decisionPoints, dp)
            }
        case "switch_expression":
            if dp := analyzeSwitchExpression(node, code, config); dp != nil {
                decisionPoints = append(decisionPoints, dp)
            }
        case "return_statement":
            if dp := analyzeReturnStatement(node, code, config); dp != nil {
                decisionPoints = append(decisionPoints, dp)
            }
        case "throw_statement":
            if dp := analyzeThrowStatement(node, code, config); dp != nil {
                decisionPoints = append(decisionPoints, dp)
            }
        case "catch_clause":
            if dp := analyzeCatchClause(node, code, config); dp != nil {
                decisionPoints = append(decisionPoints, dp)
            }
        }
    })
    
    // Insert mask markers
    return insertMasks(code, decisionPoints)
}
```

#### 2. Decision Point Analysis

Each node type has specific analysis logic:

**If Statements:**
```go
func analyzeIfStatement(node, content, config) *DecisionPoint {
    // Extract condition
    conditionNode := node.ChildByFieldName("condition")
    conditionCode := content[conditionNode.StartByte():conditionNode.EndByte()]
    
    // Calculate complexity (operators, method calls, logic depth)
    complexity := calculateComplexity(conditionCode)
    
    // Skip only if below threshold (default threshold: 1)
    if complexity < config.ComplexityThreshold {
        return nil
    }
    
    // Determine mask type
    if isValidationPattern(conditionCode) {
        return &DecisionPoint{
            Type: "validation",
            Hint: "add validation check",
            Priority: complexity * 10
        }
    } else if isBusinessLogicPattern(conditionCode) {
        return &DecisionPoint{
            Type: "business_logic",
            Hint: "implement business logic condition",
            Priority: complexity * 10
        }
    }
    
    return &DecisionPoint{
        Type: "condition",
        Hint: "complete the condition",
        Priority: complexity * 10
    }
}
```

**Return Statements:**
```go
func analyzeReturnStatement(node, content, config) *DecisionPoint {
    returnCode := content[node.StartByte():node.EndByte()]
    
    // Skip simple returns
    if returnCode == "return;" || returnCode == "return null;" {
        return nil
    }
    
    // Mask returns that meet complexity threshold
    complexity := calculateComplexity(returnCode)
    if complexity >= config.ComplexityThreshold {
        return &DecisionPoint{
            Type: "return",
            Hint: "complete the return statement",
            Priority: complexity * 8
        }
    }
    
    return nil
}
```

**Throw Statements:**
```go
func analyzeThrowStatement(node, content, config) *DecisionPoint {
    // Always mask throw statements (error handling is important)
    return &DecisionPoint{
        Type: "error",
        Hint: "add error handling",
        Priority: 7  // Medium priority
    }
}
```

**Switch Expressions:**
```go
func analyzeSwitchExpression(node, content, config) *DecisionPoint {
    // Mask entire switch if complex (> N cases)
    cases := countCases(node)
    
    if cases >= config.SwitchCaseThreshold {
        return &DecisionPoint{
            Type: "switch",
            Hint: "complete the switch cases",
            Priority: cases * 5
        }
    }
    
    return nil
}
```

**Empty Catch Blocks:**
```go
func analyzeCatchClause(node, content, config) *DecisionPoint {
    catchBody := node.ChildByFieldName("body")
    
    // Check if catch block is empty or only has comments
    if isEmpty(catchBody, content) {
        return &DecisionPoint{
            Type: "error",
            Hint: "add error handling in catch block",
            Priority: 9  // High priority (anti-pattern)
        }
    }
    
    return nil
}
```

#### 3. Complexity Calculation

```go
func calculateComplexity(code string) int {
    complexity := 0
    
    // Count logical operators
    complexity += strings.Count(code, "&&") * 2
    complexity += strings.Count(code, "||") * 2
    complexity += strings.Count(code, "!") * 1
    
    // Count comparison operators
    complexity += strings.Count(code, "==") * 1
    complexity += strings.Count(code, "!=") * 1
    complexity += strings.Count(code, ">") * 1
    complexity += strings.Count(code, "<") * 1
    
    // Count method calls (indicates logic)
    complexity += strings.Count(code, "(") * 1
    
    // Parentheses depth
    complexity += calculateNestingDepth(code) * 2
    
    return complexity
}
```

**Thresholds:**
- `ComplexityThreshold = 1`: Minimum complexity to mask (higher complexity = higher priority)
- Simple condition (`x > 5`): complexity = 1 → **Masked**
- Medium condition (`x != null && x.isEmpty()`): complexity = 4 → **Masked**
- Complex condition (`(a && b) || (c && d && !e)`): complexity = 10 → **Masked (highest priority)**

#### 4. Type-Aware Mask Marker Insertion

The mask insertion creates **syntactically valid placeholders** so the file can still be parsed:

```go
func (m *JavaMasker) insertMaskMarker(code string, point DecisionPoint) (Mask, string) {
    maskID := generateMaskID()
    maskComment := fmt.Sprintf("/* [MASK:session=%s id=%s hint=\"%s\"] */", sessionID, maskID, point.Hint)
    
    // Type-aware placeholder keeps file syntactically valid
    var placeholder string
    switch point.Type {
    case MaskTypeCondition, MaskTypeValidation:
        placeholder = fmt.Sprintf("(true %s)", maskComment)
    case MaskTypeReturn:
        placeholder = fmt.Sprintf("return null; %s", maskComment)
    case MaskTypeError:
        placeholder = fmt.Sprintf("throw new RuntimeException(); %s", maskComment)
    default:
        placeholder = maskComment
    }
    
    // Replace original code with placeholder
    before := code[:point.StartByte]
    after := code[point.EndByte:]
    maskedCode := before + placeholder + after
    
    return mask, maskedCode
}
```

**Placeholder Examples:**

| Mask Type | Original | Placeholder |
|-----------|----------|-------------|
| Condition | `(x == null \|\| x.isEmpty())` | `(true /* [MASK:session=<uuid> id=<maskId> hint="..."] */)` |
| Return | `return a + b * c;` | `return null; /* [MASK:session=<uuid> id=<maskId> hint="..."] */` |
| Throw | `throw new IllegalArgException(msg);` | `throw new RuntimeException(); /* [MASK:session=<uuid> id=<maskId> hint="..."] */` |

### Session Storage

When masking completes, a session is created in SQLite:

```sql
CREATE TABLE mask_sessions (
    id TEXT PRIMARY KEY,           -- UUID
    original_code TEXT NOT NULL,   -- Code before masking
    masked_code TEXT NOT NULL,     -- Code with MASK markers
    filled_code TEXT,              -- User's filled code (initially empty)
    file_path TEXT NOT NULL,       -- Target file path
    language TEXT NOT NULL,        -- "java"
    status TEXT NOT NULL,          -- "pending", "filled", "validated"
    created_at DATETIME,
    updated_at DATETIME
);

-- Example row:
INSERT INTO mask_sessions VALUES (
    'uuid-abc-123',
    'public void foo() { if (x > 5) return true; }',  -- original
    'public void foo() { if /* [MASK:session=<uuid> id=<maskId> hint="..."] */ return true; }',  -- masked
    NULL,  -- filled (empty initially)
    'Service.java',
    'java',
    'pending',
    '2024-01-15 10:00:00',
    '2024-01-15 10:00:00'
);
```

**Status Lifecycle:**
- `pending`: Masked code generated, waiting for user
- `filled`: User submitted filled code, validation failed
- `validated`: User submitted filled code, validation passed

---

## External Libraries & Tools

### Core Dependencies

1. **tree-sitter** (`github.com/smacker/go-tree-sitter`)
   - **Purpose**: Parse source code into AST (language plugin selected by file/language)
   - **Usage**: Identify decision points by traversing AST nodes
   - **Why**: Fast, incremental, language-agnostic parsing

2. **tree-sitter language grammars**
   - **Purpose**: Language-specific grammars (Java/Python/TypeScript/JavaScript)
   - **Usage**: Provide language-specific AST node types used by each plugin masker

3. **SQLite** (`github.com/mattn/go-sqlite3`)
   - **Purpose**: Store mask sessions
   - **Usage**: Persist original/masked/filled code for validation
   - **Table**: `mask_sessions`

4. **crypto/rand** (Go stdlib)
   - **Purpose**: Generate unique mask IDs
   - **Usage**: SHA-256 hash of (code + timestamp + random bytes)

---

## Configuration

### Environment Variables

None required. Masking is purely deterministic and self-contained.

### Masking Configuration

Adjust thresholds in code:

```go
type MaskingConfig struct {
    ComplexityThreshold  int   // Minimum complexity to mask (default: 1)
    SwitchCaseThreshold  int   // Minimum cases to mask switch (default: 3)
    EnableTODOMasking    bool  // Mask TODO comments (default: true)
    EnableEmptyCatch     bool  // Mask empty catch blocks (default: true)
}

// Default configuration:
func DefaultMaskingConfig() *MaskingConfig {
    return &MaskingConfig{
        ComplexityThreshold:  1,
        SwitchCaseThreshold:  3,
        EnableTODOMasking:    true,
        EnableEmptyCatch:     true,
    }
}
```

---

## Usage Examples

### MCP Usage (Copilot)

Masking is executed through MCP tool calls (`kg.maskCode`) while `fypd serve-mcp` is running.

```text
Example tool result payload:
{
  "session_id": "abc-123-uuid",
  "masked_code": "public void save(User u) { if /* [MASK:session=<uuid> id=mask_xyz hint=\"add validation check\"] */ { throw new Exception(); } repo.save(u); }",
  "masks": [
    {
      "id": "mask_xyz",
      "type": "validation",
      "hint": "add validation check",
      "original_code": "(u == null || u.getEmail() == null)",
      "range": {"start": {"line": 2, "character": 7}, "end": {"line": 2, "character": 45}}
    }
  ]
}
```

### Programmatic Usage

```go
import "example.com/fyp/pkg/masking/java"

// Create masker
masker := java.NewJavaMasker()

// Mask code
code := `
public boolean validate(String input) {
    if (input == null || input.isEmpty()) {
        return false;
    }
    return input.length() > 5;
}
`

result, err := masker.MaskCode(code)
if err != nil {
    log.Fatal(err)
}

fmt.Println("Masked Code:")
fmt.Println(result.MaskedCode)

fmt.Println("\nMasks:")
for _, mask := range result.Masks {
    fmt.Printf("- %s: %s (hint: %s)\n", mask.ID, mask.Type, mask.Hint)
}
```

### With Database Storage

```go
import (
    "example.com/fyp/pkg/masking/java"
    "example.com/fyp/pkg/db"
)

// Initialize database
store, _ := db.NewStore("/path/to/workspace")
defer store.Close()

// Mask code
masker := java.NewJavaMasker()
result, _ := masker.MaskCode(code)

// Store session
sessionID, err := store.CreateMaskSession(db.MaskSession{
    OriginalCode: code,
    MaskedCode:   result.MaskedCode,
    FilePath:     "Service.java",
    Language:     "java",
})

fmt.Printf("Session ID: %s\n", sessionID)
```

---

## Troubleshooting

### "No decision points found"

**Cause:** Code too simple or all conditions below complexity threshold

**Solutions:**
1. Lower `ComplexityThreshold` in config
2. Add more complex conditions to code
3. Ensure code has if/switch/return/throw statements

### "Parse error: syntax error at line X"

**Cause:** Code has syntax errors that prevent tree-sitter from parsing

**Solution:** Fix syntax errors before triggering `kg.maskCode` (for Java, validate with `javac -Xlint` first).

### "Masks overlap" or "Invalid byte offsets"

**Cause:** Multiple decision points at same location (rare)

**Solution:** Automatic de-duplication (handled by masker). If persists, check for tree-sitter version compatibility.

### "Session not found" During Validation

**Cause:** Session ID doesn't exist in database

**Check:**
```bash
# List all sessions
sqlite3 .fyp/index.db "SELECT id, file_path, status FROM mask_sessions;"
```

**Solution:** Re-run masking to create a new session.

---

## Mask Types

The system identifies 6 types of decision points:

| Type | Description | Example | Priority |
|------|-------------|---------|----------|
| `validation` | Input validation checks | `if (x == null \|\| x.isEmpty())` | High (complexity × 10) |
| `business_logic` | Business rules | `if (user.getRole() == ADMIN && order.getTotal() > 1000)` | High (complexity × 10) |
| `condition` | General conditions | `if (count > threshold)` | Medium (complexity × 10) |
| `return` | Complex returns | `return a + b * c / d` | Medium (complexity × 8) |
| `error` | Error handling | `throw new Exception()`, empty catch | High (7-9) |
| `switch` | Switch expressions | `switch (status) { ... }` | Medium (cases × 5) |

**Priority determines order of masking when multiple points exist in same scope.**

---

## Session Status Lifecycle

```
[pending]
   ↓ (user submits filled code)
   ├─→ [filled] (validation failed - user can retry)
   │      ↓ (user resubmits)
   │      ├─→ [filled] (failed again)
   │      └─→ [validated] (success!)
   │
   └─→ [validated] (validation passed)
```

**Database Tracking:**
```sql
-- View session history
SELECT id, file_path, status, created_at, updated_at 
FROM mask_sessions 
ORDER BY created_at DESC;

-- Sessions by status
SELECT status, COUNT(*) FROM mask_sessions GROUP BY status;
```

---

## Performance

- **Parsing**: ~10-50ms (tree-sitter is very fast)
- **Analysis**: ~5-20ms (AST traversal + complexity calculation)
- **Mask Insertion**: ~1-5ms (string manipulation)
- **Database Storage**: ~2-10ms (SQLite insert)

**Total:** ~20-100ms per file (typical Java class)

**Scalability:** Linear O(n) with code size (tree-sitter is incremental)

---

## Summary

✅ **What We Built:**
- Deterministic masking using tree-sitter AST analysis
- 6 decision point types (validation, business logic, conditions, returns, errors, switch)
- Complexity-based filtering (only mask non-trivial code)
- Session management with SQLite storage
- Integration with validation pipeline

✅ **Educational Value:**
- Students practice decision-making (not syntax)
- Focus on logic patterns (validation, error handling, business rules)
- Scaffolded learning (template with hints)
- Immediate feedback (validation after filling)

✅ **Key Features:**
- Zero-config (works out of the box)
- Language-agnostic architecture (extensible to Python, Go, etc.)
- Fast (tree-sitter parsing)
- Production-ready (error handling, session tracking)

---

## Related Documentation

- [VISION.md](VISION.md) - Problem statement and feature overview
- [VALIDATION.md](VALIDATION.md) - Validation pipeline details
- [MCP.md](MCP.md) - MCP integration (`kg.maskCode`, `kg.validateFilledCode`)
- [BACKEND_API_SUMMARY.md](BACKEND_API_SUMMARY.md) - Backend API overview
- [build-with-me/README.md](build-with-me/README.md) - VS Code extension details
