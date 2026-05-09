# FYP Server (fypd)

A local-first code knowledge database server for indexing and analyzing workspace codebases. The server provides symbol resolution, call graph analysis, code masking, validation, and segmentation capabilities.

## Documentation

For detailed information about each major feature, see:

- **[docs/INDEXING.md](../docs/INDEXING.md)** - Code indexing system (symbols, method calls, call graph)
- **[docs/MCP.md](../docs/MCP.md)** - Model Context Protocol server (GitHub Copilot integration)
- **[docs/MASKING.md](../docs/MASKING.md)** - Code masking system (decision points, educational exercises)
- **[docs/VALIDATION.md](../docs/VALIDATION.md)** - Code validation pipeline (parse-only)
- **[API_DOCS.md](API_DOCS.md)** - Full stdio API and MCP reference

## Building

### Prerequisites
- Go 1.25.1 or later
- CGO enabled (for SQLite)

### Build Commands

```bash
cd server
CGO_ENABLED=1 go build -o bin/fypd ./cmd/fypd

# Or use the Makefile
make build
```

## Starting the Server

The server runs as an MCP (Model Context Protocol) server that communicates via JSON-RPC over stdio, making it suitable for integration with GitHub Copilot and other AI coding assistants.

```bash
# Start the MCP server
./bin/fypd serve-mcp --workspace /path/to/workspace

# Or use current directory as workspace
./bin/fypd serve-mcp --workspace .
```

The server will listen on stdio for JSON-RPC requests. It requires a workspace directory that has been indexed first.

**Logging:** The MCP server automatically logs all feature usage to `<workspace>/.fyp/mcp-server.log`. Logs are also written to stderr for immediate visibility.

## Command Line Interface

### `serve-api-stdio`
Start the stdio JSON-RPC API server used by the VS Code extension webview bridge.

**Options:**
- `--workspace PATH` - Workspace directory (required)

**Example:**
```bash
fypd serve-api-stdio --workspace ./my-workspace
```

### `serve-mcp`
Start the MCP server for GitHub Copilot integration.

**Options:**
- `--workspace PATH` - Workspace directory (default: current directory)

**Example:**
```bash
fypd serve-mcp --workspace ./my-workspace
```

### Indexing (RPC from extension)
Workspace indexing is triggered through the stdio API:
- `index.scan` (symbol scan)
- Extension LSP call-hierarchy resolution
- `index.storeEdges` (persist symbols + edges)

`index.run` is a deprecated compatibility stub and does not perform indexing.

### `version`
Print the server version.

**Example:**
```bash
fypd version
```

### `help`
Show help message with all available commands.

**Example:**
```bash
fypd help
```

## Quick Start Workflow

1. **Build the server:**
   ```bash
   cd server
   make build
   ```

2. **Index your workspace from the extension:**
   ```bash
   # In VS Code: FYP: Open Index Panel -> Build Index
   ```

3. **Start the MCP server:**
   ```bash
   ./bin/fypd serve-mcp --workspace /path/to/workspace
   ```

## Output Format

Servers communicate over stdio JSON-RPC. Logs, errors, and progress information are written to stderr.

## Database

The server creates a SQLite database at `.fyp/index.db` in the workspace directory. This database contains:
- Symbols (classes, methods, constructors, fields)
- Method calls (call graph edges)
- File cache (for incremental indexing)
- Virtual nodes, annotations, snapshots (graph-viz data)
- Mask sessions (for masking workflow)

## Features

- **Multi-language Parsing** - Tree-sitter plugins for Java, Python, TypeScript, and JavaScript
- **Compiler-Resolved Call Graph** - LSP call hierarchy resolution orchestrated by the extension
- **Symbol Extraction** - Callable symbols with signatures/body hashes for full and incremental indexing
- **SQLite Storage** - Local `.fyp/index.db` per workspace
- **Graph Queries** - Nodes and edges for graph-viz
- **MCP Server** - JSON-RPC over stdio for GitHub Copilot integration
- **Code Masking** - Deterministic AST masking (language plugin-based)
- **Code Validation** - Parse-only validation (language plugin-based)
- **Code Segmentation** - Method segmentation for supported languages

## Version

Current version: 0.2.0
