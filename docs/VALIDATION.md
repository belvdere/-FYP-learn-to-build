# Code Validation System

## Purpose

The validation system completes the masking workflow by providing **automated verification** that filled code is syntactically valid. Parse validation catches syntax errors while keeping the system fast and dependency-free.

| Problem | How Validation Helps |
|---------|----------------------|
| Manual review is slow | Automated parse validation |
| Syntax errors in filled code | tree-sitter catches parse errors early |

---

## What It Does

The validation system verifies user-filled code through **parse-only validation**:

1. **Parse** (Blocking): Syntax validation using tree-sitter

The validator can work **with or without a session ID**:
- **With session**: Parse validation + returns original code (for MCP path; Copilot does semantic comparison)
- **Without session**: Parse only (standalone validation)

Supported validator plugins currently include Java, Python, TypeScript, and JavaScript. (Examples below use Java syntax.)

**Example Use Case:**
```
Student fills masked code → Submit for validation → Get feedback:
  ✅ Parse: Valid syntax
  
  → Overall: PASS (ready for Copilot semantic comparison, if applicable)
```

---

## Workflow

### Complete Validation Flow

```
Input: User-filled code + optional session ID
Output: Validation results (passed/failed + feedback)

┌─────────────────────────────────────────────────────────────┐
│  1. Receive Validation Request                              │
│     - file_path: "Service.java"                             │
│     - code: "public void foo() { ... }"                     │
│     - session_id: "uuid" (optional)                         │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  2. Stage: Parse (Blocking)                                 │
│     ───────────────────────────────────────                 │
│     - Parse code with tree-sitter                           │
│     - Check for syntax errors in AST                        │
│     - If errors: STOP                                        │
│     - If valid: Continue                                     │
│                                                             │
│     Result: {stage: "parse", passed: true/false, errors: []}│
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  3. Update Database (if session ID provided)                 │
│     ────────────────────────────────────                    │
│     UPDATE mask_sessions SET:                               │
│       filled_code = "...",                                  │
│       status = "validated" (if passed) or "filled" (failed) │
│       updated_at = NOW()                                    │
│     WHERE id = session_id                                   │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  4. Return Results                                          │
│     {                                                       │
│       "passed": true,                                       │
│       "results": [                                          │
│         {stage: "parse", passed: true, ...}                  │
│       ],                                                    │
│       "original_code": "..." (for MCP - Copilot comparison) │
│     }                                                       │
└─────────────────────────────────────────────────────────────┘
```

### MCP Path (Copilot)

For `kg.validateFilledCode`:
1. Run parse validation
2. Return original code from session so Copilot can perform its own semantic comparison
3. Copilot compares filled code vs original and provides feedback to user

### CodeLens Path (Extension)

For the VS Code extension (CodeLens / command palette):
1. Run parse validation only
2. Show pass/fail and diagnostics

---

## How It Works

### Parse Validation

**Purpose:** Ensure code is syntactically valid for the target language plugin

**Implementation:**
```go
func (v *JavaValidator) validateParse(code string) ValidationResult {
    // Parse with tree-sitter
    tree, err := v.parser.ParseCtx(context.Background(), nil, []byte(code))
    if err != nil {
        return ValidationResult{
            Stage: StageParse,
            Passed: false,
            Errors: []ValidationError{{Message: err.Error()}},
        }
    }
    
    // Check for parse errors in AST
    if tree.RootNode().HasError() {
        return ValidationResult{
            Stage: StageParse,
            Passed: false,
            Errors: []ValidationError{{
                Line: 1, Column: 0,
                Message: "Parse tree contains errors",
                Severity: "error",
                Code: "SYNTAX_ERROR"
            }},
            Hints: []string{"The code has syntax errors that prevent parsing"},
        }
    }
    
    return ValidationResult{
        Stage: StageParse,
        Passed: true,
        Errors: []ValidationError{},
    }
}
```

**What It Catches:**
- Missing/mismatched braces, parentheses, brackets
- Invalid keywords or operators
- Malformed expressions
- Incomplete statements

**Example:**
```java
// Invalid (missing closing brace):
public void foo() { return;

// Error: Parse tree contains errors
```

---

## External Libraries & Tools

### Core Dependencies

1. **tree-sitter** (`github.com/smacker/go-tree-sitter`)
   - **Purpose**: Parse source code for syntax validation (language plugin selected by file/language)
   - **Usage**: Parse stage
   - **Performance**: ~50-100ms

2. **tree-sitter language grammars**
   - **Purpose**: Language-specific grammars (Java/Python/TypeScript/JavaScript)
   - **Usage**: Provide AST node types for each validator plugin

3. **SQLite** (`github.com/mattn/go-sqlite3`)
   - **Purpose**: Store/retrieve mask sessions (for original code retrieval on MCP path)
   - **Usage**: Fetch original code to return for Copilot comparison

---

## Configuration

### Environment Variables

None required. Parse-only validation is self-contained with no external API dependencies.

### Validation Configuration

```go
// Validator interface - parse-only
type JavaValidator struct {
    parser *sitter.Parser
}

// Create validator
validator := java.NewJavaValidator()
```

---

## Usage Examples

### API Validation (Stdio RPC)

```json
{"id": 1, "method": "validation.run", "params": {"filePath": "Service.java", "code": "public void foo() { return; }"}}
{"id": 2, "method": "validation.run", "params": {"filePath": "Service.java", "code": "public void foo() { if (x > 5) return true; }", "sessionId": "uuid-abc-123"}}
```

**Output:**
```json
{
  "passed": true,
  "results": [
    {
      "stage": "parse",
      "passed": true,
      "errors": []
    }
  ]
}
```

### Programmatic Usage

```go
import "example.com/fyp/pkg/validation/java"

// Create validator
validator := java.NewJavaValidator()

// Validate code
code := `
public boolean isValid(String input) {
    if (input == null) return false;
    return input.length() > 5;
}
`

results, err := validator.ValidateFilledCode("Service.java", code)
if err != nil {
    log.Fatal(err)
}

// Check overall result
passed := checkOverallPass(results)
fmt.Printf("Validation: %s\n", map[bool]string{true: "PASS", false: "FAIL"}[passed])

// Print stage details
for _, result := range results {
    fmt.Printf("\n%s: %s\n", result.Stage, map[bool]string{true: "✅", false: "❌"}[result.Passed])
    for _, err := range result.Errors {
        fmt.Printf("  - %s (line %d)\n", err.Message, err.Line)
    }
}
```

---

## Troubleshooting

### "Parse stage failed" but Code Looks Valid

**Cause:** Code might be a fragment (not a complete class/method)

**Solution:** Wrap in a class:
```java
// Instead of:
if (x > 5) return true;

// Use:
public class Test {
    public boolean foo() {
        if (x > 5) return true;
        return false;
    }
}
```

---

## Performance

### Typical Latency

| Stage | Latency | Notes |
|-------|---------|-------|
| Parse | 50-100ms | tree-sitter (very fast) |
| **Total** | **50-100ms** | No external dependencies |

---

## Summary

✅ **What We Built:**
- Parse-only validation pipeline
- Session-aware validation (returns original code for MCP/Copilot comparison)
- Graceful fallbacks (works with/without session ID)

✅ **Performance:**
- **Fast**: 50-100ms (parse only)
- **Scalable**: Efficient tree-sitter parsing, no external API calls

✅ **Key Features:**
- Backward compatible (works with/without session ID)
- Production-ready (error handling, logging, metrics)
- Zero external dependencies (no API keys required)

---

## Related Documentation

- [VISION.md](VISION.md) - Problem statement and feature overview
- [MASKING.md](MASKING.md) - Masking system details
- [MCP.md](MCP.md) - MCP integration details
- [BACKEND_API_SUMMARY.md](BACKEND_API_SUMMARY.md) - Backend API overview
- [build-with-me/README.md](build-with-me/README.md) - VS Code extension details
