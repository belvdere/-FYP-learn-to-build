# Backend API Summary

## Purpose

`fypd` is the local backend used by the VS Code extension and Copilot MCP integration.

Core responsibilities:
- Symbol + call-edge storage for graph visualization
- Graph CRUD (nodes, edges, annotations, snapshots)
- Masking/validation support

## Runtime Architecture

- VS Code extension starts `fypd serve-api-stdio --workspace <path>`.
- Extension talks to backend with JSON-RPC over stdio.
- Copilot uses `fypd serve-mcp --workspace <path>` for MCP tools.

## Indexing API (Current)

| Method | Description |
|--------|-------------|
| `index.scan` | Scan source files and return symbols with positions for LSP resolution |
| `index.scanFile` | Scan one file and return symbols + changed/removed callers + file hash |
| `index.storeEdges` | Persist resolved symbols and call edges |
| `index.appendEdges` | Append additional resolved edge batches |
| `index.resolveSymbolsByLocation` | Resolve `filePath + line` locations to canonical symbol IDs |
| `index.storeFileDelta` | Apply on-save incremental symbol/edge updates for one file |
| `index.clearSymbols` | Clear symbols and method calls |
| `index.run` | Deprecated no-op compatibility stub |

Notes:
- LSP call resolution runs in the extension, not in `fypd`.
- On full workspace indexing, extension may call `index.storeEdges` then `index.appendEdges` in batches.
- On-save indexing uses `index.scanFile` + LSP + `index.storeFileDelta`.

## Graph and Metadata API

| Domain | Methods |
|--------|---------|
| Graph | `graph.get`, `graph.neighborhood`, `graph.search`, `graph.clearVirtual` |
| Nodes | `nodes.list/get/create/update/delete` |
| Edges | `edges.list/create/delete` |
| Annotations | `annotations.node.get/update`, `annotations.edge.get/update` |
| Virtual dirs | `virtual.directories.list/create/delete` |
| Snapshots | `snapshots.list/create/get/load/delete`, `snapshots.prompt.get/update/reset` |
| Validation | `validation.run` |

## Handler Layout

`server/pkg/api/`:
- `stdio_server.go` request router
- `handlers_index_lsp.go` indexing handlers
- `handlers_graph.go`, `handlers_node.go`, `handlers_edge.go`
- `handlers_virtual.go`, `handlers_snapshot.go`, `handlers_validation.go`
- `file_tree.go`

## Database Model (Indexing-Relevant)

Key tables:
- `symbols(id, name, kind, file_path, line, signature, created_at)`
- `method_calls(caller_id, callee_id, call_type, file_path, line, created_at)`
- `file_cache(...)`

Canonical edge columns are `caller_id` and `callee_id`.

## CLI

Runtime server entry points:

```bash
./bin/fypd serve-api-stdio --workspace <path>
./bin/fypd serve-mcp --workspace <path>
```
