# Model Context Protocol (MCP) Integration

## Purpose

The MCP server enables **GitHub Copilot** to access project-specific context for code generation. This addresses the **control gap** in agentic AI workflows by allowing developers to provide explicit, bounded context via graph snapshots.

## How It Fits the Vision

| Problem | Solution |
|---------|----------|
| AI context is implicit and uncontrollable | Snapshots provide explicit context via `kg.getSnapshot` |
| Generated code ignores project conventions | Annotations guide AI to match patterns |
| Developers blindly accept AI output | `kg.maskCode` + `kg.validateFilledCode` triggers learning workflow |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  GitHub Copilot (Agentic Mode)                              │
│  "Generate the virtual nodes in snapshot snap_abc123 --mask"│
└───────────────────────────┬─────────────────────────────────┘
                            │ MCP (JSON-RPC 2.0 over stdio)
                            ▼
┌─────────────────────────────────────────────────────────────┐
│  fypd serve-mcp                                             │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  MCP Server (server/pkg/mcp/)                        │   │
│  │  • initialize: Handshake with Copilot                │   │
│  │  • tools/list: Advertise available tools             │   │
│  │  • tools/call: kg.getSnapshot, kg.maskCode,          │   │
│  │                 kg.validateFilledCode                │   │
│  └─────────────────────────────────────────────────────┘    │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  Prompt Optimizer (optional, FYP_OPTIMIZE_PROMPTS=1)│   │
│  │  • Python subprocess: prompt-optimizer/optimize.py   │   │
│  │  • TextGrad refines snapshot markdown pre-Copilot    │   │
│  └─────────────────────────────────────────────────────┘    │
│                            ↓                                │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  SQLite Database (.fyp/index.db)                     │   │
│  │  • Snapshots (virtual nodes, context nodes, edges)   │   │
│  │  • Annotations (descriptions, AI remarks)            │   │
│  │  • Mask sessions (for kg.maskCode / validation)      │   │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

---

## Available Tools

### `kg.getSnapshot`

Retrieve a graph snapshot by ID. Returns rich context about virtual nodes to generate and existing nodes for reference.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `snapshotId` | string | Yes | Snapshot ID (e.g., "snap_abc123") |

**Returns:** Markdown-formatted context including:
- **Virtual Design** — rendered as a directory → class → method hierarchy. Each virtual directory, class, and method/concept is shown with its description, AI remarks, and relationships to other nodes.
- **Context Nodes** — existing code for reference, grouped by file
- **Relationships** — how virtual and real components connect
- AI remarks and conventions from annotations

**Example Workflow:**
```
1. Developer creates virtual directory "services/payment" in graph-viz
2. Inside it, creates virtual class "PaymentService" (description: "Handles payment processing")
3. Inside PaymentService, creates virtual method "processPayment" with AI remarks
4. Draws edge from "processPayment" → existing "PaymentGateway.charge" (context node)
5. Captures snapshot → snap_abc123
6. Prompts Copilot: "Generate the virtual nodes in snapshot snap_abc123"
7. Copilot calls kg.getSnapshot → receives full hierarchical context
8. Generates code following documented patterns
```

### `kg.maskCode`

Mask generated code for a learning exercise. Returns masked code with syntactically-valid placeholders that Copilot writes to the target file.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `code` | string | Yes | The generated Java code |
| `filePath` | string | Yes | Target file path |
| `language` | string | No | Programming language (default: "java") |

**Returns:** Masked code with `[MASK]` placeholders and session IDs for validation. The masked code uses type-aware placeholders to keep the file syntactically valid:
- Conditions: `(true /* [MASK:id=X hint="..."] */)`
- Returns: `return null; /* [MASK:id=X hint="..."] */`
- Throws: `throw new RuntimeException(); /* [MASK:id=X hint="..."] */`

**Example Workflow:**
```
1. User prompts: "Create a UserService class with CRUD methods --mask"
2. Copilot generates complete code
3. Copilot calls kg.maskCode with the generated code
4. Backend segments code → masks decision points → stores session → returns masked code
5. Copilot writes masked code to the target file
6. User fills in [MASK] placeholders in their normal editor
7. User says "validate" → Copilot calls kg.validateFilledCode
```

### `kg.validateFilledCode`

Run parse validation on user-filled masked code. Returns the original code so Copilot can perform its own semantic comparison.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `sessionId` | string | Yes | The mask session ID from `kg.maskCode` |
| `filledCode` | string | Yes | The user's filled code (read from file) |
| `filePath` | string | Yes | The file path being validated |

**Returns:** Parse validation results (pass/fail) and the original code for Copilot to compare semantically.

**Example Workflow:**
```
1. User says "validate my code"
2. Copilot reads the file containing the filled masks
3. Copilot calls kg.validateFilledCode with the session ID and code
4. Backend runs parse validation (syntax check via tree-sitter)
5. Returns parse results and original code to Copilot
6. Copilot performs semantic comparison and reports results to user (without revealing original code)
```

### Prompt Optimizer (optional)

When `FYP_OPTIMIZE_PROMPTS=1` is set, the snapshot markdown returned by `kg.getSnapshot` is refined by a **TextGrad-based optimizer** before being sent to Copilot. This improves clarity and reduces ambiguity in the code generation context.

| Env | Description | Default |
|-----|-------------|---------|
| `FYP_OPTIMIZE_PROMPTS` | Set to `1` to enable | Off |
| `FYP_PROMPT_OPTIMIZER_PATH` | Path to `optimize.py` | Auto-resolved |
| `FYP_OPTIMIZE_TIMEOUT` | Subprocess timeout (seconds) | 30 |

**Flow:** After fetching the snapshot and formatting it as markdown, the MCP server spawns `python3 optimize.py --steps 1`, pipes the content on stdin, and reads optimized content from stdout. On failure (missing Python, timeout, etc.), it falls back to the original content and logs a warning.

See [TEXTGRAD_INTEGRATION.md](TEXTGRAD_INTEGRATION.md) for setup, API key, and offline scripts.

---

## Implementation

### Server Structure (`server/pkg/mcp/`)

```go
// server.go
type Server struct {
    store          *db.Store
    langController *lang.LanguageController  // Language plugins (.java → Java, etc.)
    reader         *bufio.Reader             // stdin (newline-delimited JSON-RPC)
    writer         io.Writer                 // stdout
    logFile        *os.File                  // .fyp/mcp-server.log (append)
    logMutex       sync.Mutex
    workspaceRoot string
}
```

**Serve loop:** Reads newline-delimited JSON-RPC requests from stdin, unmarshals into `Request`, and routes via `handleRequest`. Methods: `initialize`, `resources/list`, `resources/read`, `tools/list`, `tools/call`. All responses are JSON-RPC 2.0 over stdout.

**Logging:** Every tool call and status is logged to stderr and `mcp-server.log` via `logFeatureUsage(feature, status, details)`.

### kg.getSnapshot Implementation

1. Extract `snapshotId` from args; return error if missing
2. Fetch snapshot from `store.GetSnapshot(snapshotID)`
3. Format content: use `snapshot.CustomPrompt` if set, else `types.FormatSnapshotAsMarkdown(snapshot)`
   - `FormatSnapshotAsMarkdown` (`pkg/types/snapshot_formatter.go`) renders virtual nodes as a **directory → class → method tree** using each `SnapshotNode.ParentID`. Nodes without parents fall to a flat fallback list.
4. **If `FYP_OPTIMIZE_PROMPTS=1`:** call `runPromptOptimizer(workspaceRoot, markdownContent)`; on success replace content; on failure fall back to original and log
5. Return markdown as `ToolResult` text content

### kg.maskCode Implementation

1. Extract `code`, `filePath`; validate; get language plugin via `GetPlugin(filePath)`
2. **Segment:** `segmenter.SegmentClass(code)` → `[]MethodSegment` (tree-sitter `method_declaration` nodes)
3. **Mask each method:** `masker.MaskCodeWithStore(method.Body, filePath#methodName, store)` → masked body + session ID; on error use original body and new UUID
4. **Reassemble:** `reassembleCode(code, methods, maskedMethodBodies)` — replace method bodies by line range (bottom-to-top to avoid index shifts)
5. Resolve `filePath` to absolute using `workspaceRoot` if relative
6. Return markdown with: absolute path, method list (name, session ID, lines), instructions, and code block

**`reassembleCode`:** Splits original code into lines, processes methods from last to first, replaces `lines[startIdx:endIdx+1]` with masked body lines, joins back.

### kg.validateFilledCode Implementation

1. Extract `sessionId`, `filledCode`, `filePath`; get language plugin
2. `validator.ValidateFilledCode(filePath, filledCode)` → parse-only tree-sitter validation
3. `store.GetMaskSession(sessionID)` → original code for Copilot comparison
4. `store.UpdateFilledCode` and `store.UpdateStatus`
5. Build response: parse pass/fail, errors if any, then `Original Code (for your comparison)` block (only if parse passed) — Copilot uses this for semantic comparison without revealing to user

### Prompt Optimizer Implementation (`server/pkg/mcp/optimizer.go`)

- **`runPromptOptimizer(workspaceRoot, content)`** — runs `python3 <path> --steps 1`:
  - Stdin: raw content
  - Stdout: optimized content
  - Env: `mergedEnv(workspaceRoot)` — merges `os.Environ()` with `workspaceRoot/.fyp/.env` (for `OPENAI_API_KEY`)
  - Timeout: `FYP_OPTIMIZE_TIMEOUT` seconds (default 30)
  - On failure: returns error; caller falls back to original

- **`resolveOptimizerPath(workspaceRoot)`** — search order:
  1. `FYP_PROMPT_OPTIMIZER_PATH` if set
  2. `{workspace}/prompt-optimizer/optimize.py`
  3. `{workspace}/.fyp/prompt-optimizer/optimize.py`
  4. `{execDir}/../prompt-optimizer/optimize.py` (packaged extension)

- **`loadEnvFromFile`** — parses `.env` (KEY=value, ignores comments and empty lines)
- **`mergedEnv`** — overlays .env vars over existing env; .env wins on conflicts

### Prompt Optimizer Scripts (`prompt-optimizer/`)

| Script | Purpose |
|--------|---------|
| `optimize.py` | Per-call: refines snapshot markdown via TextGrad; reads stdin, writes stdout |

**`optimize.py` internals:** Loads `.fyp/.env` for `OPENAI_API_KEY`; uses `textgrad.Variable` with role "code generation context"; `TextLoss` asks "Does it clearly specify what to implement? Are virtual nodes unambiguous?"; 1 TGD step (configurable via `--steps`).

### JSON-RPC Protocol (`protocol.go`)

```
Request:  { "jsonrpc": "2.0", "id": <any>, "method": "...", "params": {...} }
Response: { "jsonrpc": "2.0", "id": <any>, "result": {...} }
Error:    { "jsonrpc": "2.0", "id": <any>, "error": { "code": int, "message": str } }
```

Tool results use `ToolResult{ Content: []ContentItem{{ Type: "text", Text: "..." }}, IsError: bool }`.

### Language Controller

```go
plugin, err := s.langController.GetPlugin(filePath)  // ".java" → JavaPlugin

// Per-tool access
masker := plugin.Masker()       // tree-sitter masking (decision points)
segmenter := plugin.Segmenter() // tree-sitter segmentation (method_declaration)
validator := plugin.Validator()  // tree-sitter parse-only validation
```

---

## Configuration

### VS Code Extension Setup

The extension registers the MCP server with Copilot:

```json
// mcp.json (generated by extension)
{
  "mcpServers": {
    "build-with-me": {
      "command": "/path/to/fypd",
      "args": ["serve-mcp", "--workspace", "/path/to/project"]
    }
  }
}
```

Run `FYP: Configure Copilot MCP` command to set this up.

### Logging

- **stderr**: Real-time console output
- **Log file**: `<workspace>/.fyp/mcp-server.log`

```
[2024-01-15 10:30:00] [MCP] [TOOL] kg.getSnapshot called with snapshotId=snap_abc123
[2024-01-15 10:30:01] [MCP] [TOOL] Returning 3 virtual nodes, 5 context nodes
```

---

## Usage Examples

### Snapshot-Based Generation

```
User: "Generate the virtual nodes in snapshot snap_abc123"

Copilot:
1. Parses prompt, extracts snapshot ID
2. Calls kg.getSnapshot(snapshotId="snap_abc123")
3. Receives markdown context (hierarchical):

   ## Virtual Design
   ### 📁 services/order *(virtual directory)*
   #### 📦 OrderService *(virtual class)*
   - Description: Manages order lifecycle
   ##### OrderService.cancelOrder *(virtual method)*
   - AI Remarks: "Use @Transactional, follow refundOrder pattern"
   - Related: → PaymentService.processRefund

   ## Context Nodes (Existing Code)
   File: src/services/PaymentService.java
   - PaymentService.processRefund (line 42) — method: reference pattern

4. Generates code following the documented hierarchy and patterns
```

### Masked Generation

```
User: "Create an AuthService with login/logout methods --mask"

Copilot:
1. Generates complete AuthService class
2. Detects --mask flag
3. Calls kg.maskCode(code="...", filePath="AuthService.java")
4. Receives masked code with [MASK] placeholders and session IDs
5. Writes masked code to AuthService.java
6. Tells user: "Fill in the [MASK] placeholders, then say 'validate'"

User fills in masks, then: "validate my code"

Copilot:
7. Reads AuthService.java
8. Calls kg.validateFilledCode(sessionId="...", filledCode="...", filePath="...")
9. Reports results: "2/3 masks correct. Check the return statement hint."
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `pkg/db` | SQLite queries for snapshots, mask sessions |
| `pkg/lang` | Language controller for multi-language support |
| `pkg/masking` | Tree-sitter AST masking (decision points) |
| `pkg/segmentation` | Tree-sitter AST method segmentation |
| `pkg/validation` | Tree-sitter parse-only validation |
| `pkg/mcp/optimizer.go` | Prompt optimizer subprocess (Python TextGrad) |
| `prompt-optimizer/` | Python scripts: `optimize.py`, tool descriptions, copilot instructions |

---

## Related Documentation

- [VISION.md](VISION.md) - Problem statement and feature overview
- [MASKING.md](MASKING.md) - Masking system details
- [VALIDATION.md](VALIDATION.md) - Validation pipeline details
- [TEXTGRAD_INTEGRATION.md](TEXTGRAD_INTEGRATION.md) - Prompt optimizer setup, API key, offline scripts
- [AST_FEATURES.md](AST_FEATURES.md) - How AST powers masking, validation, symbols, segmentation
- [COMPONENT_ARCHITECTURE.md](COMPONENT_ARCHITECTURE.md) - Graph-viz, extension, and backend communication
- [server/API_DOCS.md](../server/API_DOCS.md) - Full API reference
