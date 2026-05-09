# User Flows: What to Expect

This document describes the end-to-end flows for each of the four core capabilities—indexing, masking, validation, and visualization—from the user's perspective, with sequence diagrams showing the interactions between user, extension, Copilot, and backend.

---

## 1. Indexing

**User goal:** Build a local knowledge base of the codebase (symbols, references, call graph) for visualization and other features.

### What the User Does

1. Open Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`) → run **FYP: Open Index Panel**
2. In Index Panel, wait until language server status is ready
3. Click **Build Index** (or **Rebuild Index** for a clean re-index)

### What Happens

- Phase 1 (Go): `index.scan` extracts symbols and positions
- Phase 2 (Extension + LSP): per-symbol call hierarchy resolves outgoing calls
- Phase 3 (Go): `index.storeEdges` writes canonical symbols/edges to `.fyp/index.db`
- Progress appears in the Index Panel (scan → lsp → store)

### Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant Extension
    participant Fypd
    participant Database

    User->>Extension: FYP: Open Index Panel + Build Index
    Extension->>Fypd: index.scan (JSON-RPC stdio)
    Fypd-->>Extension: symbols with source positions
    loop each symbol
      Extension->>Extension: vscode.prepareCallHierarchy
      Extension->>Extension: vscode.provideOutgoingCalls
    end
    Extension->>Fypd: index.storeEdges (symbols + edges)
    Fypd->>Database: Replace index data in .fyp/index.db
    Fypd-->>Extension: success + stats
    Extension-->>User: Index complete
```

---

## 2. Masking

**User goal:** Get AI-generated code with decision points replaced by `[MASK]` placeholders so they can fill in critical logic themselves.

### What the User Does

1. In Copilot chat: include `--mask` in the prompt (e.g., "Create a UserService with CRUD methods --mask")
2. Copilot generates code, masks it, and writes the masked file
3. User fills in the `[MASK]` placeholders in the editor
4. User validates (see Validation flow)

### What Happens

- Copilot sends generated code to `kg.maskCode` (MCP tool)
- Backend parses code, finds decision points (conditionals, returns, etc.), and replaces them with type-aware placeholders and mask markers
- Session is stored in the database; session ID is embedded in mask comments
- Copilot writes the masked code to the file (using the absolute path from the response)
- User edits the file to replace placeholders with real logic

### Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant Copilot
    participant FypdMCP
    participant Database

    User->>Copilot: "Create UserService.save() --mask"
    Copilot->>Copilot: Generate code
    Copilot->>FypdMCP: kg.maskCode(code, filePath)
    FypdMCP->>FypdMCP: Parse AST, segment methods
    FypdMCP->>FypdMCP: Find decision points, insert masks
    FypdMCP->>Database: Store session (original, masked)
    FypdMCP-->>Copilot: maskedCode, sessionId, filePath
    Copilot->>User: Write masked file (create_file)
    Copilot-->>User: "Fill in the [MASK] placeholders. Say 'validate' when done."
    User->>User: Edit file, replace placeholders
```

---

## 3. Validation

**User goal:** Check that their filled code is syntactically valid (and, via Copilot, semantically correct).

Validation can be triggered in two ways:

### Path A: Via Copilot (MCP)

When the user says "validate" or "check my masks":

```mermaid
sequenceDiagram
    participant User
    participant Copilot
    participant FypdMCP
    participant Database

    User->>Copilot: "validate"
    Copilot->>User: Read current file
    Copilot->>FypdMCP: kg.validateFilledCode(sessionId, filledCode, filePath)
    FypdMCP->>FypdMCP: Parse validation (tree-sitter)
    FypdMCP->>Database: GetMaskSession(sessionId)
    FypdMCP->>Database: Update filled_code, status
    FypdMCP-->>Copilot: Parse results + original code
    Note over Copilot: Copilot compares filled vs original
    Copilot-->>User: Feedback (hints if wrong, no answer reveal)
```

### Path B: Via CodeLens (Extension)

When the user clicks **Validate All Masks** or **Validate Filled Code** above the file:

```mermaid
sequenceDiagram
    participant User
    participant Extension
    participant FypdAPI
    participant Database

    User->>Extension: Click "Validate" CodeLens
    Extension->>FypdAPI: validation.run (RPC)
    FypdAPI->>FypdAPI: Parse validation only
    FypdAPI->>Database: Update filled_code, status
    FypdAPI-->>Extension: passed, results
    Extension->>Extension: Set diagnostics (if errors)
    Extension-->>User: Problems panel + notification
```

**What the user sees:**

- **Parse pass:** "All masks validated successfully"
- **Parse fail:** Errors in the Problems panel with line numbers
- **Via Copilot only:** Semantic feedback (hints vs. correct) based on comparison with original code

---

## 4. Visualization

**User goal:** Explore the call graph, design virtual architecture, annotate nodes/edges, create snapshots for AI context.

### What the User Does

1. Run **FYP: Open Graph Editor** (Command Palette or sidebar)
2. (If needed) Build or Rebuild index first from **FYP: Open Index Panel**
3. **Populate the canvas** — nodes are not shown automatically; add them explicitly:
   - Click **Load All** to pre-load all graph data into memory (still not on canvas)
   - Click **⊕** next to any node in the file tree to add it to the canvas
   - **Double-click** a node on the canvas to expand and add its call-graph neighbors
   - **Right-click** a method → adds its parent class node; right-click a class → adds its parent directory node
4. **Annotate** nodes/edges: single-click a node → InfoPanel opens → click ✏️ Edit → fill in description / AI remarks → Save
5. **Design virtual architecture** using the Shape Palette toolbar above the canvas:
   - Click **⬜ Virtual Directory** → click canvas to place an orange dashed box
   - Click **⬛ Virtual Class** → click canvas to place a purple dashed box
   - Click **◇ Virtual Method** → click canvas to place a purple diamond
   - Escape cancels placement mode
   - Double-click any node (real or virtual) to expand neighborhood
6. **Draw edges** — hover a node until the edge handle appears, then drag to another node
7. **Resize nodes** — select a node, then drag any of the four corner handles
8. **Toggle layers** — use the **Dir / File / Method** buttons in the canvas toolbar to show/hide node groups
9. **Delete nodes/edges** — select then press `Delete` or `Backspace` (virtual nodes are removed from the backend; real nodes are removed from the canvas only)
10. **Clear graph planning state** — use **Clear** to remove all virtual nodes/directories and manual edges from DB state
11. Create snapshots for Copilot (e.g., "Generate the virtual nodes in snapshot snap_abc123")
12. Load a snapshot from **Saved Snapshots → Load** to materialize snapshot state back into the graph

### What Happens

- Extension opens a webview panel and starts the API stdio server if needed
- Webview requests graph data from the backend
- Backend reads from the index database, aggregates symbols and method calls
- User interacts with Cytoscape.js graph; virtual/manual changes are saved to the database
- Loading a snapshot first clears current virtual/manual state, then restores the snapshot's saved virtual nodes/edges

### Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant Extension
    participant Webview
    participant FypdAPI
    participant Database

    User->>Extension: FYP: Open Graph Editor
    Extension->>FypdAPI: Start serve-api-stdio
    Extension->>Webview: Create panel, load graph-viz
    Webview->>Extension: postMessage apiRequest
    Extension->>FypdAPI: Forward JSON-RPC
    FypdAPI->>Database: Query symbols, method_calls, virtual_nodes
    FypdAPI-->>Extension: graph.nodes, graph.edges
    Extension-->>Webview: Response
    Webview-->>User: Render graph (Cytoscape.js)

    User->>Webview: Click ⊕ on file tree node
    Note over Webview: Looks up GraphNode by filePath,<br/>adds to displayGraphData
    Webview-->>User: Node appears on canvas

    User->>Webview: Double-click real node to expand
    Webview->>Extension: graph.neighborhood (apiRequest)
    Extension->>FypdAPI: graph.neighborhood RPC
    FypdAPI->>Database: Fetch neighbors
    FypdAPI-->>Extension: New nodes/edges
    Webview-->>User: Neighbor nodes appear on canvas

    User->>Webview: Right-click method node
    Note over Webview: Finds parent class via filePath,<br/>adds class node to canvas

    User->>Webview: Select node, click ✏️ Edit, fill in description/AI remarks
    Webview->>Extension: annotations.node.update
    Extension->>FypdAPI: Forward RPC
    FypdAPI->>Database: Store annotation
    Note over Webview: Canvas label updates immediately

    User->>Webview: Click shape in palette, click canvas
    Webview->>Extension: nodes.create (apiRequest)
    Extension->>FypdAPI: Forward RPC
    FypdAPI->>Database: Insert virtual node
    Webview-->>User: Virtual node placed at clicked position

    User->>Webview: Drag edge handle from node A to node B
    Webview->>Extension: edges.create (apiRequest)
    Extension->>FypdAPI: Forward RPC
    FypdAPI->>Database: Insert edge
    Webview-->>User: Edge drawn (manual edges render in brown by default)

    User->>Webview: Capture snapshot (📸 Snapshot tab)
    Webview->>Extension: snapshots.create
    Extension->>FypdAPI: snapshots.create RPC
    FypdAPI->>Database: Store snapshot (with virtual/context nodes + edges)
    FypdAPI-->>Extension: snapshot ID (e.g., snap_abc123)
    Webview-->>User: "Snapshot created: snap_abc123"
```

---

See [COMPONENT_ARCHITECTURE.md](COMPONENT_ARCHITECTURE.md) for how the graph-viz webview, extension, and backend communicate.

## Summary

| Flow | User Trigger | Backend | User Outcome |
|------|--------------|---------|--------------|
| **Indexing** | Open Index Panel → Build/Rebuild | `index.scan` + LSP + `index.storeEdges` | Call graph in `.fyp/index.db` |
| **Masking** | "Create X --mask" (Copilot) | kg.maskCode (MCP) | Masked file with placeholders |
| **Validation** | "validate" (Copilot) or CodeLens | kg.validateFilledCode / validation.run | Parse results, optional semantic feedback |
| **Visualization** | Open Graph Editor | graph.* RPCs, nodes.*, edges.*, annotations.*, snapshots.* | Flat draw.io canvas, virtual node placement, edge drawing, annotations, snapshots |
