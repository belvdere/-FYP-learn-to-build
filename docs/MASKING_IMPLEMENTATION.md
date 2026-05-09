# Masking and Validation: Backend Implementation Details

This document covers the backend implementation of the masking and validation system, focusing on how decisions are made, how code is transformed, and how validation is structured.

## Table of Contents

1. [Masking Pipeline](#masking-pipeline)
2. [How We Decide What to Mask](#how-we-decide-what-to-mask)
3. [Heuristic Ranking: Priority Scoring](#heuristic-ranking-priority-scoring)
4. [How Masks Are Inserted and Returned](#how-masks-are-inserted-and-returned)
5. [Validation Pipeline](#validation-pipeline)
6. [Pipeline Summary Diagrams](#pipeline-summary-diagrams)

---

## Masking Pipeline

The masking pipeline transforms valid Java source code into a partially-masked version where critical decision points are replaced with type-aware placeholders. This runs entirely on the backend (Go) with no LLM dependency -- masking is **deterministic and AST-driven**.

### Entry Point

The pipeline is triggered via the `kg.maskCode` MCP tool, which receives `code` and `filePath` from Copilot.

### Pipeline Steps

```
Copilot sends code + filePath
        │
        ▼
┌─────────────────────────┐
│  1. Language Detection   │   Resolve language plugin from file extension
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│  2. Segmentation         │   Parse AST → extract individual methods
│     (per class)          │   Each method becomes an independent unit
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│  3. Masking              │   For each method:
│     (per method)         │   a. Parse method body → AST
│                          │   b. Traverse AST → find decision points
│                          │   c. Score and rank by priority
│                          │   d. Select top N (max 3)
│                          │   e. Insert type-aware placeholders
│                          │   f. Store session in database
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│  4. Reassembly           │   Replace original method bodies with
│                          │   masked versions in the full class
└───────────┬─────────────┘
            ▼
┌─────────────────────────┐
│  5. Response             │   Return masked code + session IDs
│                          │   to Copilot (who writes it to file)
└─────────────────────────┘
```

#### Step 1: Language Detection

The `langController.GetPlugin(filePath)` resolves the file extension to a language plugin. Currently only Java is supported. The plugin provides three components:
- **Segmenter**: Splits a class into methods
- **Masker**: Identifies and masks decision points
- **Validator**: Validates filled code

#### Step 2: Segmentation

The `JavaSegmenter.SegmentClass(code)` function:

1. Parses the entire class with tree-sitter
2. Walks the AST to find all `method_declaration` nodes
3. For each method, extracts:
   - Name, signature, return type, parameters
   - Body (full source text of the method)
   - Start/end line numbers (1-indexed, for reassembly)

This step is necessary because masking operates at the **method level** -- each method gets its own mask session and its own set of decision points (capped at `MaxMasksPerFunction`).

#### Step 3: Masking (per method)

For each method, `JavaMasker.MaskCodeWithStore(methodBody, filePath, store)` runs:

1. **Parse**: tree-sitter parses the method body into an AST
2. **Identify**: Traverse AST to find decision points (detailed below)
3. **Sort**: Rank decision points by priority (descending)
4. **Limit**: Take top `MaxMasksPerFunction` (default: 3)
5. **Insert**: Replace decision point code with type-aware placeholders (from end to start to preserve byte offsets)
6. **Inject session ID**: Rewrite `/* [MASK:id=...]` to `/* [MASK:session=<uuid> id=...]`
7. **Store**: Save a `MaskSession` record with original code, masked code, and status `pending`

#### Step 4: Reassembly

`reassembleCode(originalCode, methods, maskedBodies)` replaces original method bodies with their masked versions:

- Splits the full class into lines
- Processes methods **bottom to top** (to avoid line-number shifting)
- For each method, replaces lines `[startLine, endLine]` with the masked version
- Joins lines back into the final output

#### Step 5: Response

The MCP handler returns a structured markdown response to Copilot containing:
- The absolute file path (so Copilot can write the file)
- The list of methods with their session IDs
- The complete masked code in a code block
- Instructions for Copilot: write the file, tell user to fill masks, validate when asked

---

## How We Decide What to Mask

Decision point identification is **purely AST-based** -- no LLM, no regex, no string matching on the raw code. The `identifyDecisionPoints` function traverses the tree-sitter AST and examines specific node types.

### AST Node Types Targeted

| AST Node Type | Maps To | What It Masks |
|---|---|---|
| `if_statement` | Condition/Validation | The parenthesized condition expression |
| `switch_expression` / `switch_statement` | Condition/Validation | The switch expression |
| `ternary_expression` / `conditional_expression` | Condition/Validation | The condition part (before `?`) |
| `return_statement` | Return | The entire return statement |
| `throw_statement` | Error | The entire throw statement |

### Per-Node Analysis

Each node type has a dedicated analysis function that determines **whether** to mask and **what exactly** to mask:

#### `analyzeIfStatement`

1. Extract the `condition` child node (the parenthesized expression)
2. Calculate complexity of the condition code
3. If `complexity < ComplexityThreshold` (default: 1) → **skip** (too simple)
4. Check if the condition is a validation pattern → set type to `validation` with hint "add validation check"
5. Otherwise → set type to `condition` with hint "complete the condition"
6. The **masked region** is the condition node only (not the if body)

#### `analyzeReturnStatement`

1. Extract the full return statement text
2. **Skip** simple returns: `return;` and `return null;`
3. Calculate complexity of the return expression
4. If `complexity < ComplexityThreshold` → **skip**
5. The **masked region** is the entire return statement

#### `analyzeThrowStatement`

1. Extract the full throw statement text
2. Always masks (no complexity filtering) -- error handling is always educational
3. Fixed priority of 7
4. The **masked region** is the entire throw statement

#### `analyzeSwitchStatement`

1. Walk children to find the switch expression (parenthesized expression, identifier, field access, or method invocation)
2. Fall back to `condition` field name, then first non-keyword child
3. Calculate complexity (minimum of 1 for switches)
4. Check for validation pattern
5. The **masked region** is the switch expression only

#### `analyzeTernaryExpression`

1. Walk children to find the condition (first child before the `?` token)
2. Fall back to `condition` field name
3. Calculate complexity (minimum of 1 for ternaries)
4. Check for validation pattern
5. The **masked region** is the condition part only

### Configuration Flags

The `MaskingConfig` controls which node types are even considered:

```go
MaskValidation: true   // if, switch, ternary → enabled
MaskReturns:    false  // return statements → disabled by default
MaskErrors:     true   // throw statements → enabled
```

This means by default, the masker focuses on **conditional logic and error handling** rather than return expressions.

---

## Heuristic Ranking: Priority Scoring

After all decision points are identified, they are ranked by a **priority score** to determine which are most educationally valuable. Only the top `MaxMasksPerFunction` (default: 3) are selected.

### Priority Formula

```
priority = complexity × type_multiplier
```

Where:

| Decision Point Type | Type Multiplier | Rationale |
|---|---|---|
| `if_statement` condition | × 10 | Conditions in if-statements are the most common and important decision points |
| `switch_statement` expression | × 9 | Switch logic is important but slightly less frequent |
| `return_statement` | × 8 | Complex returns require understanding data flow |
| `ternary_expression` condition | × 8 | Ternaries are concise decisions, same value as returns |
| `throw_statement` | 7 (fixed) | Error handling is always relevant but fixed priority |

### Complexity Calculation

The `calculateComplexity(code)` function counts structural indicators in the code text:

```
complexity = count("&&")     // logical AND
           + count("||")     // logical OR
           + count("==")     // equality
           + count("!=")     // inequality
           + count(">")      // comparison
           + count("<")      // comparison
           + count("(") / 2  // method calls (approximation)
           + count(".")      // member access / chaining
```

**Examples:**

```java
// complexity = 0 (would be skipped if threshold > 0)
if (flag) { ... }

// complexity = 3 (null=1 "==" + isEmpty=1 "." + "||"=1)
// But actually: "==" counts 1, "||" counts 1, "." for isEmpty() counts 1, "(" counts 0.5
// Total ≈ 3-4 → priority = 3 × 10 = 30
if (input == null || input.isEmpty()) { ... }

// complexity ≈ 6+ → priority = 60+
if (user != null && user.getAge() >= 18 && user.isActive()) { ... }
```

### Validation Pattern Detection

Before assigning the mask type, the code is checked against validation keywords:

```go
keywords: "null", "isEmpty", "isBlank", "length", "valid", "check", "verify", "assert"
```

If any keyword is found (case-insensitive), the mask type is set to `validation` instead of `condition`. This affects the **hint** shown to the user ("add validation check" vs. "complete the condition") but does not change the priority score.

### Selection Process

```
1. Collect all decision points from AST traversal
2. Sort by priority (descending)
3. Take first MaxMasksPerFunction (default: 3)
```

This ensures the **most complex and educationally valuable** decision points are masked, while simpler ones are left visible as context.

---

## How Masks Are Inserted and Returned

### Type-Aware Placeholder Strategy

A key design constraint: **the masked code must remain syntactically valid Java**. This is critical because:
- The file is written to disk and opened in VS Code
- Linters, syntax highlighters, and tree-sitter must still work
- The user edits the file in-place, replacing placeholders with real code

Each mask type uses a different placeholder that maintains validity:

| Mask Type | Original Code | Placeholder |
|---|---|---|
| `condition` / `validation` | `(input == null \|\| input.isEmpty())` | `(true /* [MASK:session=... id=... hint="..."] */)` |
| `return` | `return input.length() > 5;` | `return null; /* [MASK:session=... id=... hint="..."] */` |
| `error` | `throw new IllegalArgumentException("bad");` | `throw new RuntimeException(); /* [MASK:session=... id=... hint="..."] */` |

The placeholder has two parts:
1. **A valid Java expression** (`true`, `null`, `new RuntimeException()`) that satisfies the compiler
2. **A mask comment** containing the session ID, mask ID, and hint for the user

### Insertion Order

Masks are inserted **from end to start** (reverse byte offset order) to preserve byte positions:

```go
for i := len(decisionPoints) - 1; i >= 0; i-- {
    point := decisionPoints[i]
    mask, newCode := m.insertMaskMarker(maskedCode, point)
    maskedCode = newCode
}
```

This is necessary because inserting a placeholder at an earlier position shifts all subsequent byte offsets.

### What Is Returned to the Frontend

The MCP tool response to Copilot contains:

```markdown
## Masked Code Ready

**File (absolute path):** /path/to/UserService.java
**Methods masked:** 2
- `validate` (session: abc-123, lines 10-20)
- `process` (session: def-456, lines 25-40)

### Instructions
1. Write the masked code below to the absolute file path above
2. Tell the user to fill in the [MASK] placeholders
3. When the user says "validate", call kg.validateFilledCode with the session IDs

### Masked Code
```java
public class UserService {
    public boolean validate(String input) {
        if (true /* [MASK:session=abc-123 id=mask_a1b2c3 hint="add validation check"] */) {
            return false;
        }
        return input.length() > 5;
    }
    // ...
}
```

Copilot then:
1. Writes this file to disk
2. Tells the user to replace the `(true /* [MASK:...] */)` with their own conditions
3. When the user says "validate", Copilot reads the file and calls `kg.validateFilledCode`

### Session Storage

Each masked method creates a `mask_sessions` record:

| Column | Value |
|---|---|
| `id` | UUID (e.g., `abc-123`) |
| `file_path` | `UserService.java#validate` |
| `original_code` | The original method body (before masking) |
| `masked_code` | The masked method body (with placeholders) |
| `language` | `java` |
| `status` | `pending` |
| `filled_code` | NULL (populated after validation) |

---

## Validation Pipeline

Validation checks whether the user's filled code is syntactically correct using **parse-only validation**. For the MCP path, the original code is also returned so Copilot can perform semantic comparison itself.

### Entry Point

The `kg.validateFilledCode` MCP tool receives `sessionId`, `filledCode`, and `filePath`.

### Pipeline Steps

```
Copilot sends sessionId + filledCode + filePath
        │
        ▼
┌─────────────────────────────────┐
│  1. Language Detection           │   Resolve plugin from file extension
└───────────┬─────────────────────┘
            ▼
┌─────────────────────────────────┐
│  2. Create Validator             │   Parse-only validator (tree-sitter)
└───────────┬─────────────────────┘
            ▼
┌─────────────────────────────────┐
│  3. Parse Validation (BLOCKING)  │   tree-sitter parses filled code
│                                  │   Check for syntax errors
│                                  │   ❌ Fail → return immediately
│                                  │   ✅ Pass → continue
└───────────┬─────────────────────┘
            ▼
┌─────────────────────────────────┐
│  4. Fetch Original Code          │   (MCP path only)
│     from session in DB           │   Returns original code so Copilot
│                                  │   can do semantic comparison itself
└───────────┬─────────────────────┘
            ▼
┌─────────────────────────────────┐
│  5. Update Database              │   Store filled code
│                                  │   Set status: "filled"
└───────────┬─────────────────────┘
            ▼
┌─────────────────────────────────┐
│  6. Response to Copilot          │   Parse results + original code
│                                  │   Copilot compares semantically
└─────────────────────────────────┘
```

### Stage 1: Parse Validation (Blocking)

The `validateParse(code)` function:

1. **Empty check**: Rejects empty/whitespace-only code immediately
2. **Tree-sitter parse**: Parses the filled code as Java
3. **Error detection**: Checks `root.HasError()` on the AST
4. **Error location**: If errors found, walks the AST to find the first `ERROR` node and extracts line/column
5. **Code snippet**: Extracts the offending line for context

**This stage is blocking** -- if the code has syntax errors, validation stops immediately.

**Example failure response:**

```
Stage: parse [FAIL]
- Error: Syntax error near line 5:12: if (input == null ||)
- Hint: The code has syntax errors that prevent parsing
```

### Semantic Comparison (Copilot)

For the MCP path (`kg.validateFilledCode`), after parse validation passes, the backend fetches the original code from the session database and includes it in the response. Copilot (which is already an LLM) then compares the user's filled code against the original to determine semantic correctness. This eliminates the need for a separate LLM API call.

For the CodeLens path (extension), only parse validation runs — no semantic comparison is performed.

### Overall Pass Logic

The `checkValidationPass(results)` function determines the final outcome:

```
Parse stage: MUST pass (blocking)
Overall = parsePassed
```

- If parse **fails** → return `false`
- If parse **passes** → return `true`

### Database Updates

After validation completes:

1. **Store filled code**: `UpdateFilledCode(sessionId, filledCode)` saves the user's attempt
2. **Update status**:
   - All stages passed → status = `"validated"`
   - Any stage failed → status = `"filled"` (attempted but not correct)

### Response to Copilot (MCP Path)

The response is formatted markdown with parse results and the original code:

**On parse success:**
```markdown
### Stage: parse [PASS]

## Parse Passed
The code is syntactically valid.

## Original Code (for your comparison)
Compare the user's filled code against the original below to determine if it is semantically correct.
Do NOT reveal the original code to the user. Guide them with hints if their implementation differs.

\`\`\`
<original code here>
\`\`\`
```

**On parse failure:**
```markdown
### Stage: parse [FAIL]
- Error: Syntax error near line 5:12: if (input == null ||)

## Parse Failed
The code has syntax errors. Guide the user to fix the syntax issues above.
```

The response instructs Copilot to **not** show the original code, preserving the educational value. Copilot performs its own semantic comparison using the included original code.

---

## Pipeline Summary Diagrams

### Masking Pipeline (Deterministic, No LLM)

```
Source Code (.java)
       │
       ▼
  tree-sitter Parse ──→ AST
       │
       ▼
  Segment into methods
       │
       ▼
  For each method:
       │
       ├─→ Parse method body → AST
       │
       ├─→ Traverse AST nodes
       │     │
       │     ├─ if_statement     → analyzeIfStatement()     → priority = complexity × 10
       │     ├─ switch_statement → analyzeSwitchStatement()  → priority = complexity × 9
       │     ├─ ternary_expr    → analyzeTernaryExpression() → priority = complexity × 8
       │     ├─ return_statement → analyzeReturnStatement()  → priority = complexity × 8
       │     └─ throw_statement  → analyzeThrowStatement()   → priority = 7 (fixed)
       │
       ├─→ Filter: complexity >= threshold?
       │
       ├─→ Sort by priority (descending)
       │
       ├─→ Take top 3 (MaxMasksPerFunction)
       │
       ├─→ Insert placeholders (end-to-start)
       │     │
       │     ├─ condition → (true /* [MASK:...] */)
       │     ├─ return   → return null; /* [MASK:...] */
       │     └─ error    → throw new RuntimeException(); /* [MASK:...] */
       │
       └─→ Store session (original + masked → DB)
       │
       ▼
  Reassemble full class (masked method bodies replace originals)
       │
       ▼
  Return to Copilot → Copilot writes file to disk
```

### Validation Pipeline (Parse-Only + Copilot Comparison)

```
Filled Code + Session ID
       │
       ▼
  Parse Validation (tree-sitter)
       │
       ├─ FAIL → Return errors, stop
       │
       ▼ PASS
  Fetch original code from session DB
       │
       ▼
  Update DB: filled_code → user's code
       │
       ▼
  Return parse results + original code to Copilot
       │
       ▼
  Copilot compares semantically (MCP path)
  or user sees parse results only (CodeLens path)
```
