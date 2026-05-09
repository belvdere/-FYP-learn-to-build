# Build-With-Me

A VS Code extension that augments GitHub Copilot's agentic workflow to improve developer control and confidence over AI-assisted code changes.

## Problem Statement

Agent-mode AI pair programming is increasingly embedded into IDE workflows, where an assistant can identify relevant files, propose multi-file code changes, and iterate until a task is complete. While this increases capability, developers still face a **trust-and-ownership gap** when adopting AI-generated changes in real codebases—particularly in mature repositories where small inconsistencies with project conventions can create hidden integration issues.

### Core Question

> How can we augment an agentic AI pair-programming workflow to preserve productivity gains while improving developer control and confidence over AI-assisted code changes?

## Solution: Three Core Features

Build-With-Me extends GitHub Copilot with three features designed to bridge the trust-and-ownership gap:

| Feature | Purpose | Addresses |
|---------|---------|-----------|
| **Masking + Validation** | Constrain AI output and increase developer ownership | Trust gap |
| **Method-Call Visualization** | Improve structural understanding of the codebase | Verification overhead |
| **Graph Snapshots** | Control AI's task scope with explicit context | Control gap |

---

### 1. Masking + Validation

AI-generated code is analyzed for "decision points" (conditionals, returns, error handling) using tree-sitter AST parsing. These are replaced with mask markers, forcing developers to write critical logic themselves.

```java
// AI generates:
public void deleteUser(String id) {
    if (id == null || id.isEmpty()) {
        throw new ValidationException("Invalid user ID");
    }
    userRepository.delete(id);
}

// Developer receives (masked):
public void deleteUser(String id) {
    if /* [MASK:id=1 hint="validate input"] */ {
        throw new ValidationException("Invalid user ID");
    }
    userRepository.delete(id);
}
```

**Validation Pipeline:**

| Stage | Type | What It Checks |
|-------|------|----------------|
| Parse | Blocking | Syntax validity (tree-sitter) |

---

### 2. Method-Call Visualization

Interactive graph visualization for exploring and annotating the codebase:

```
┌─────────────┬──────────────────────────┬─────────────────┐
│  File Tree  │      Graph Canvas        │   Info Panel    │
│             │                          │                 │
│  📁 src/    │     ┌─────┐              │  Selected:      │
│   📁 user/  │     │Save │──────┐       │  UserService    │
│    UserSvc  │     └─────┘      │       │  .save()        │
│    UserRepo │         │        ▼       │                 │
│             │         ▼    ┌──────┐    │  Description:   │
│             │     ┌──────┐ │Delete│    │  [editable]     │
│             │     │ Find │ └──────┘    │                 │
│             │     └──────┘             │  AI Remarks:    │
│             │                          │  [editable]     │
└─────────────┴──────────────────────────┴─────────────────┘
```

- **Explore:** Click to select, double-click to expand neighbors
- **Annotate:** Add descriptions and AI remarks to nodes/edges
- **Plan:** Create virtual nodes for future features

---

### 3. Graph Snapshots for AI Context

Capture a task-relevant subgraph to provide explicit, bounded context for AI code generation:

1. Select relevant nodes in the graph visualization
2. Create virtual nodes for planned features
3. Add annotations documenting conventions
4. Capture snapshot → `snap_abc123`
5. Reference in Copilot: *"Generate the virtual nodes in snapshot snap_abc123 --mask"*

The MCP tool `kg.getSnapshot` provides the full context to GitHub Copilot, ensuring generated code follows documented patterns.

---

## User Flows

See [docs/USER_FLOWS.md](docs/USER_FLOWS.md) for sequence diagrams showing what happens for indexing, masking, validation, and visualization from the user's perspective.

---

## Demo Videos

**Interactive Feature with Graph-Viz**

[![Interactive Feature with Graph-Viz](https://img.youtube.com/vi/yOSfUFT_7I0/hqdefault.jpg)](https://youtu.be/yOSfUFT_7I0)

**Building Graph and Snapshot with Graph-Viz**

[![Building Graph and Snapshot with Graph-Viz](https://img.youtube.com/vi/4s8BYksYvaw/hqdefault.jpg)](https://youtu.be/4s8BYksYvaw)

**Masking and Validation**

[![Masking and Validation](https://img.youtube.com/vi/kBvvDgUP8GY/hqdefault.jpg)](https://youtu.be/kBvvDgUP8GY)

---

## How the Features Work Together

```
┌─────────────────────────────────────────────────────────────────┐
│  1. UNDERSTAND: Method-Call Visualization                       │
│     • Index codebase → build call graph                         │
│     • Explore existing architecture                             │
│     • Annotate with conventions and patterns                    │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  2. CONTROL: Graph Snapshots                                    │
│     • Create virtual nodes for planned features                 │
│     • Select context nodes for reference                        │
│     • Capture snapshot with annotations                         │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│  3. VERIFY: Masking + Validation                                │
│     • AI generates code following snapshot context              │
│     • Decision points are masked                                │
│     • Developer fills critical logic                            │
│     • Parse validation confirms correctness                     │
└─────────────────────────────────────────────────────────────────┘
                              ↓
                    ┌─────────────────┐
                    │  Trusted Code   │
                    │  • Understood   │
                    │  • Validated    │
                    │  • Owned        │
                    └─────────────────┘
```

---

## Project Structure

```
learn-to-build/
├── server/              # Go backend (fypd)
│   ├── pkg/api/         # JSON-RPC stdio API
│   ├── pkg/mcp/         # MCP server for GitHub Copilot
│   ├── pkg/parser/      # Tree-sitter language plugins (Java/Python/TS/JS)
│   ├── pkg/indexer/     # Code indexing pipeline
│   ├── pkg/masking/     # Code masking engine
│   └── pkg/validation/  # Multi-stage validation
├── graph-viz/           # React graph visualization app
│   ├── src/components/  # UI components (FileTree, GraphCanvas, InfoPanel)
│   ├── src/hooks/       # State management hooks
│   └── src/services/    # API client
├── build-with-me/       # VS Code extension
│   ├── src/features/    # Masking sidebar, validation
│   └── src/ui/          # Webview panels
└── build-script/        # Build automation scripts
```

---

## Quick Start

### Prerequisites

- Go 1.21+
- Node.js 18+
- pnpm (`npm install -g pnpm`)

---

## Build Scripts Reference

| Script | Use Case 
|--------|----------
| `./build-script/deploy.sh` | First time setup, clean rebuild 
| `./build-script/quick-build.sh` | Regular development 
| `./build-script/build-backend.sh` | Go code changes only 
| `./build-script/build-frontend.sh` | React/TypeScript changes only

See [build-script/BUILD_SCRIPTS_README.md](build-script/BUILD_SCRIPTS_README.md) for detailed usage.

## Run the Extension

1. Open the project in VS Code
2. Press `F5` to launch Extension Development Host
3. Open a workspace with supported source files (`.java`, `.py`, `.ts/.tsx`, `.js/.jsx`, `.go`)
4. Use the Build-With-Me sidebar to index, visualize, and mask code

---

## Technology Stack

**Backend (Go):**
- Tree-sitter for AST parsing, masking, validation, and symbol scanning
- LSP-resolved callgraph orchestration via extension (`prepareCallHierarchy`/`provideOutgoingCalls`)
- SQLite for local storage (code index, annotations, snapshots)
- JSON-RPC over stdio for extension communication
- MCP server for GitHub Copilot integration

**Frontend (TypeScript + React):**
- Cytoscape.js for graph visualization
- VS Code extension API
- Masked code is edited in the standard VS Code editor (no separate editor)

**AI Integration:**
- GitHub Copilot (agentic workflow)

---

## Documentation

| Component | Documentation |
|-----------|---------------|
| Vision & Design | [VISION.md](VISION.md) |
| Masking System | [MASKING.md](MASKING.md) |
| Validation Pipeline | [VALIDATION.md](VALIDATION.md) |
| MCP Integration | [MCP.md](MCP.md) |
| Backend API | [docs/BACKEND_API_SUMMARY.md](docs/BACKEND_API_SUMMARY.md) |
| Graph Visualization | [graph-viz/README.md](graph-viz/README.md) |
| VS Code Extension | [build-with-me/README.md](build-with-me/README.md) |
---
