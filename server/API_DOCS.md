# Backend API Documentation

## Overview

The fypd backend provides two JSON-RPC interfaces over stdio:

1. **Stdio API Server** - For the graph-viz webview (JSON-RPC over stdio)
2. **MCP Server** - For GitHub Copilot integration (MCP protocol over stdio)

Both servers use newline-delimited JSON for communication.

---

## Stdio API Server (for VS Code Webview)

Start with: `./bin/fypd serve-api-stdio --workspace <path>`

The stdio API server communicates via JSON-RPC over stdin/stdout.

### Protocol

**Request:**
```json
{"id": 1, "method": "<method>", "params": {...}}
```

**Response:**
```json
{"id": 1, "result": {...}}
```

**Error:**
```json
{"id": 1, "error": {"code": -32603, "message": "...", "data": "..."}}
```

### Available Methods

#### Health & Database
| Method | Params | Description |
|--------|--------|-------------|
| `health.check` | - | Health check |
| `db.refresh` | - | Refresh database connection (after external changes) |

#### Indexing
| Method | Params | Description |
|--------|--------|-------------|
| `index.scan` | `rebuild?` (bool) | Scan source files and return callable symbols for LSP resolution. |
| `index.scanFile` | `filePath` | Scan a single file and return symbols + changed/removed caller IDs + file hash. |
| `index.resolveSymbolsByLocation` | `locations[]` | Resolve `filePath+line` locations to canonical symbol IDs. |
| `index.storeEdges` | `symbols`, `edges`, `rebuild?` | Persist LSP-resolved callgraph edges and scanned symbols (initial batch write). |
| `index.appendEdges` | `edges` | Append additional resolved edge batches without touching symbols. |
| `index.storeFileDelta` | `filePath`, `fileHash`, `symbols`, `changedCallerIds`, `removedCallerIds`, `edges` | Apply an incremental on-save file delta update. |
| `index.clearSymbols` | - | Clear indexed symbols and call edges. |
| `index.run` | `rebuild?` (bool) | Deprecated no-op compatibility stub. |

Progress for indexing is managed by the extension (Index Panel) while orchestrating scan/LSP/store phases.

#### Validation
| Method | Params | Description |
|--------|--------|-------------|
| `validation.run` | `code`, `filePath`, `sessionId`, `stages?` | Validate filled masked code (parse-only validation) |

**Response:**
```json
{
  "passed": true,
  "results": [
    {"stage": "parse", "passed": true, "errors": [], "warnings": [], ...}
  ]
}
```

**Note:** Masking is available via MCP tools (`kg.maskCode`, `kg.validateFilledCode`), not the stdio API.

#### Graph
| Method | Params | Description |
|--------|--------|-------------|
| `graph.get` | - | Get full graph data (nodes, edges, files) |
| `graph.neighborhood` | `nodeId` | Get node with neighbors |
| `graph.search` | `q`, `limit?` | Search nodes by label |
| `graph.clearVirtual` | - | Delete all virtual nodes/directories and manual edges/annotations |

#### Nodes
| Method | Params | Description |
|--------|--------|-------------|
| `nodes.list` | - | List all nodes |
| `nodes.get` | `id` | Get node by ID |
| `nodes.create` | `label`, `type`, `virtualClassId?`, `virtualDirectory?` | Create virtual node |
| `nodes.update` | `id`, `label?`, `type?`, ... | Update virtual node |
| `nodes.delete` | `id` | Delete virtual node |

#### Edges
| Method | Params | Description |
|--------|--------|-------------|
| `edges.list` | - | List all edges |
| `edges.create` | `sourceNodeId`, `targetNodeId`, `remarks?` | Create edge |
| `edges.delete` | `id` | Delete edge |

#### Annotations
| Method | Params | Description |
|--------|--------|-------------|
| `annotations.node.get` | `nodeId` | Get node annotation |
| `annotations.node.update` | `nodeId`, `description?`, `aiRemarks?`, `codeSnippet?` | Update node annotation |
| `annotations.edge.get` | `edgeId` | Get edge annotation |
| `annotations.edge.update` | `edgeId`, `remarks` | Update edge annotation |

#### File Tree
| Method | Params | Description |
|--------|--------|-------------|
| `filetree.get` | - | Get file tree structure |

#### Virtual Containers
| Method | Params | Description |
|--------|--------|-------------|
| `virtual.directories.list` | - | List virtual directories |
| `virtual.directories.create` | `path`, `name`, `parentPath?` | Create virtual directory |
| `virtual.directories.delete` | `id` | Delete virtual directory |

#### Snapshots
| Method | Params | Description |
|--------|--------|-------------|
| `snapshots.list` | - | List all snapshots |
| `snapshots.create` | `name?`, `virtualNodes`, `contextNodes`, `edges` | Create snapshot |
| `snapshots.get` | `id` | Get snapshot details |
| `snapshots.load` | `id` | Materialize snapshot virtual state back into graph DB |
| `snapshots.delete` | `id` | Delete snapshot |
| `snapshots.prompt.get` | `id` | Get snapshot prompt |
| `snapshots.prompt.update` | `id`, `prompt` | Update custom prompt |
| `snapshots.prompt.reset` | `id` | Reset to generated prompt |

---

## MCP Server (for GitHub Copilot)

Start with: `./bin/fypd serve-mcp --workspace <path>`

The MCP server implements the Model Context Protocol (JSON-RPC 2.0 over stdio).

### Protocol

**Request:**
```json
{"jsonrpc":"2.0","id":1,"method":"<method>","params":{...}}
```

**Response:**
```json
{"jsonrpc":"2.0","id":1,"result":{...}}
```

### MCP Methods

| Method | Description |
|--------|-------------|
| `initialize` | Handshake with client |
| `tools/list` | List available tools |
| `tools/call` | Invoke a tool |

### Available Tools

#### `kg.getSnapshot`

Retrieve a graph snapshot for code generation context.

**Arguments:**
- `snapshotId` (string, required): Snapshot ID (e.g., "snap_abc123")

**Example:**
```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kg.getSnapshot","arguments":{"snapshotId":"snap_abc123"}}}
```

**Returns:** Markdown with virtual nodes, context nodes, relationships, and instructions.

#### `kg.maskCode`

Send generated code through the masking pipeline. Returns masked code with type-aware placeholders.

**Arguments:**
- `code` (string, required): Generated Java code
- `filePath` (string, required): Target file path
- `language` (string, optional): Language (default: "java")

**Example:**
```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kg.maskCode","arguments":{"code":"public class MyService {...}","filePath":"src/MyService.java"}}}
```

**Returns:** Markdown text containing the absolute file path, method list with session IDs, instructions, and a code block with the masked code. Copilot parses this and writes the masked code to the target file.

#### `kg.validateFilledCode`

Run parse validation on user-filled masked code. Returns the original code so Copilot can perform its own semantic comparison.

**Arguments:**
- `sessionId` (string, required): Session ID from `kg.maskCode`
- `filledCode` (string, required): The code with masks filled in by the user
- `filePath` (string, required): File path for language detection

**Example:**
```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"kg.validateFilledCode","arguments":{"sessionId":"sess_abc","filledCode":"...","filePath":"src/MyService.java"}}}
```

**Returns:** Parse validation results (pass/fail) and original code for Copilot semantic comparison.

---

## CLI Commands

> **Note:** The VS Code extension uses RPC methods over stdio. Runtime CLI commands are server startup commands only.

### Servers
```bash
# Start MCP server for GitHub Copilot
./bin/fypd serve-mcp --workspace <path>

# Start Stdio API server for VS Code webview
./bin/fypd serve-api-stdio --workspace <path>
```

### Indexing
```bash
# RPC flow (used by extension)
{"id": 1, "method": "index.scan", "params": {"rebuild": false}}
{"id": 2, "method": "index.storeEdges", "params": {"symbols": [...], "edges": [...], "rebuild": false}}
{"id": 3, "method": "index.appendEdges", "params": {"edges": [...]} }
{"id": 4, "method": "index.scanFile", "params": {"filePath": "/abs/path/src/user/UserService.java"}}
{"id": 5, "method": "index.storeFileDelta", "params": {"filePath": "/abs/path/src/user/UserService.java", "fileHash": "...", "symbols": [...], "changedCallerIds": [...], "removedCallerIds": [...], "edges": [...]}}
```

### Masking
Masking is available via **MCP tools** (`kg.maskCode`), not the stdio API. Use `fypd serve-mcp` for Copilot integration.

### Validation
```bash
# RPC (used by extension)
{"id": 1, "method": "validation.run", "params": {"code": "...", "filePath": "...", "sessionId": "..."}}
```

---

## Logging

- **Stdio API Server**: Logs to stderr
- **MCP Server**: Logs to stderr and `<workspace>/.fyp/mcp-server.log`
