# Component Architecture: Graph-Viz Webview, VS Code Extension, and Backend

This document explains how the **graph-viz webview**, **VS Code extension**, and **backend (fypd)** communicate, how each component is created, who orchestrates them, and how the orchestrator does its job.

---

## Overview

| Component | Technology | Role |
|-----------|------------|------|
| **Graph-Viz Webview** | React + Cytoscape.js (bundled in extension) | UI for graph editing, snapshots, virtual node placement, edge drawing, layer visibility, node resizing, annotations |
| **VS Code Extension** | TypeScript (build-with-me) | **Orchestrator** — creates components, bridges messages, manages lifecycle |
| **Backend** | Go (fypd) | Indexing, graph data, SQLite, stdio API server |

**Orchestrator:** The **VS Code extension** is the orchestrator. It owns the lifecycle of the backend process and webview panel, and routes all communication between them.

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│  VS Code Extension (Orchestrator)                                       │
│  build-with-me/src/extension.ts                                         │
│                                                                         │
│  On activate:                                                           │
│    • startApiStdioServer()  ────────► spawn fypd serve-api-stdio         │
│    • autoConfigureMcpIfNeeded()                                          │
│                                                                         │
│  On "FYP: Open Graph Editor":                                           │
│    • openGraphPanel()      ────────► createWebviewPanel + load graph-viz │
│    • graphPanel.ts: onDidReceiveMessage ───► handleApiRequest ──────────►│
└────────────────────────────────────┬───────────────────┬───────────────┘
                                      │                   │
          postMessage                 │                   │  sendApiRequest
          (apiRequest / apiResponse)  │                   │  (JSON-RPC over stdin)
                                      ▼                   ▼
┌─────────────────────────┐    ┌──────────────────────────────────────────┐
│  Graph-Viz Webview      │    │  Backend (fypd serve-api-stdio)           │
│  graph-viz/ (React)      │    │  server/pkg/api/                          │
│                         │    │                                            │
│  • webviewTransport.ts   │    │  • JSON-RPC: stdin (requests)             │
│    └─ postMessage ──────┼────┤    stdout (responses)                       │
│    └─ window.addEventListener │  • Methods: graph.get, nodes.create,      │
│  • graphApi.ts           │    │    snapshots.create, graph.get, etc.      │
│  • Cytoscape.js render   │    │  • SQLite (.fyp/index.db)                 │
│    (flat draw.io canvas) │    │                                            │
└─────────────────────────┘    └──────────────────────────────────────────┘
```

---

## 1. How Each Component Is Created

### Backend (fypd)

**Created by:** VS Code extension (orchestrator).

**When:** On extension activation (`activate()` in `extension.ts`), and optionally before opening the graph panel if not already running.

**How:**

1. Extension calls `startApiStdioServer(context)`
2. Resolves fypd binary via `getFypdBinaryPath(extensionPath)`:
   - Bundled: `{extension}/bin/{darwin-arm64|darwin-x64|...}/fypd`
   - Dev: `{repo}/server/bin/fypd`
3. Spawns: `spawnFypd(bin, ['serve-api-stdio', '--workspace', workspaceRoot], ...)`
4. Process uses `stdio: ['pipe', 'pipe', 'pipe']` — extension reads stdout, writes stdin

**Lifecycle:** Runs until extension deactivation (`deactivate()` → `stopApiStdioServer()`).

### Graph-Viz Webview

**Created by:** VS Code extension (orchestrator).

**When:** User runs **FYP: Open Graph Editor** (or equivalent command).

**How:**

1. Extension calls `openGraphPanel(context)` in `graphPanel.ts`
2. If API server not running, calls `startApiStdioServer(context)` first
3. Creates panel: `vscode.window.createWebviewPanel('fypGraph', 'FYP Graph Editor', ...)`
4. Sets `webview.html = getGraphHtml(...)` — loads bundled React app from `media/graph-viz/assets/`
5. Injects `window.__USE_STDIO_API__ = true` so graph-viz uses message passing (not HTTP)
6. Registers `webview.onDidReceiveMessage` to handle `apiRequest`, `showInfo`, `showError`, `ready`

**Lifecycle:** Panel exists until user closes it. Context is retained when hidden (`retainContextWhenHidden: true`).

### VS Code Extension

**Created by:** VS Code, when the extension is activated (e.g., workspace opened, extension enabled).

**How:**

1. VS Code loads the extension and calls `activate(context)`
2. Extension:
   - Creates status bar item
   - Registers tree view (`IndexStatusProvider`)
   - Registers commands (`fyp.openGraph`, `fyp.openIndexPanel`, etc.)
   - Starts API stdio server
   - Registers mask CodeLens and file watchers

---

## 2. The Orchestrator: VS Code Extension

The extension orchestrates the system by:

1. **Owning process lifecycle** — starts and stops `fypd serve-api-stdio`
2. **Owning webview lifecycle** — creates and disposes the graph panel
3. **Bridging communication** — webview ↔ backend; extension is the only path between them
4. **Coordinating features** — indexing, MCP config, masking CodeLens, graph UI

### Orchestrator Responsibilities

| Responsibility | Implementation |
|----------------|----------------|
| **Start backend** | `startApiStdioServer()` on activate; before graph panel if needed |
| **Route API requests** | `handleApiRequest()` receives `apiRequest` from webview → `sendApiRequest()` to backend → `postMessage(apiResponse)` to webview |
| **Provide config to webview** | On `ready` message, sends `{ type: 'config', useStdioApi: true }` |
| **Handle non-API messages** | `showInfo`, `showError` → VS Code notifications |
| **Index management** | Index Panel flow: `index.scan` → LSP call hierarchy in extension → `index.storeEdges` + `index.appendEdges` (batched); on-save uses `index.scanFile` + `index.storeFileDelta` |
| **MCP configuration** | `autoConfigureMcpIfNeeded()`, `configureCopilotMcpForWorkspace()` |
| **Shutdown** | `deactivate()` → `stopApiStdioServer()`, `stopMCPServer()` |

---

## 3. Communication Flows

### Webview ↔ Extension (postMessage)

The webview runs in an isolated context. It cannot access Node.js or spawn processes. All backend access goes through the extension.

**Webview → Extension**

```typescript
// graph-viz: webviewTransport.ts
acquireVsCodeApi().postMessage({
  type: 'apiRequest',
  id: 1,
  method: 'graph.get',
  params: {}
});
```

**Extension → Webview**

```typescript
// graphPanel.ts: handleApiRequest
panel.webview.postMessage({
  type: 'apiResponse',
  id: 1,
  result: { nodes: [...], edges: [...] }
});
```

**Message types:**

| Type | Direction | Purpose |
|------|-----------|---------|
| `apiRequest` | Webview → Extension | Request backend method |
| `apiResponse` | Extension → Webview | Backend result or error |
| `config` | Extension → Webview | Tell webview to use stdio API |
| `ready` | Webview → Extension | Webview loaded, request config |
| `showInfo`, `showError` | Webview → Extension | Show VS Code notifications |

### Extension ↔ Backend (stdio JSON-RPC)

The backend speaks JSON-RPC over stdin/stdout. The extension is the only process that talks to it.

**Extension → Backend**

```
Extension writes to process.stdin (newline-delimited JSON):
{"id":1,"method":"graph.get","params":{}}
```

**Backend → Extension**

```
Backend writes to process.stdout:
{"id":1,"result":{"nodes":[...],"edges":[...],"files":[...]}}

Notifications (no id):
{"method":"index.progress","params":{"message":"Parsing...","percent":30}}
```

**Request/response correlation:** Extension maintains `pendingRequests: Map<id, {resolve, reject}>`. When stdout yields a line with `id`, it resolves the matching promise.

---

## 4. End-to-End Request Flow

Example: User opens graph, graph-viz loads and fetches data.

```
1. User: FYP: Open Graph Editor

2. Extension: openGraphPanel()
   - startApiStdioServer() if needed
   - createWebviewPanel()
   - set webview.html (loads graph-viz React app)

3. Graph-viz loads, mounts React
   - window.__USE_STDIO_API__ = true
   - webviewTransport creates WebviewTransport singleton
   - Sends postMessage({ type: 'ready' })

4. Extension receives 'ready'
   - postMessage({ type: 'config', useStdioApi: true })

5. Graph-viz (useGraph, graphApi) calls getGraphData()
   - sendRequest('graph.get', {})
   - postMessage({ type: 'apiRequest', id: 1, method: 'graph.get', params: {} })

6. Extension receives apiRequest
   - handleApiRequest() → sendApiRequest('graph.get', {})
   - Writes to fypd stdin: {"id":1,"method":"graph.get","params":{}}\n

7. Backend receives request
   - Reads from stdin, parses JSON
   - Handles graph.get → queries SQLite
   - Writes to stdout: {"id":1,"result":{...}}\n

8. Extension receives stdout line
   - handleMessage() → handleResponse()
   - pendingRequests.get(1).resolve(result)
   - postMessage({ type: 'apiResponse', id: 1, result: {...} })

9. Graph-viz receives apiResponse
   - window message listener → handleResponse()
   - pending.get(1).resolve(result)
   - React state updates → Cytoscape re-renders
```

---

## 5. Component Creation Order

```
VS Code loads extension
       │
       ▼
activate(context)
       │
       ├──► startApiStdioServer()  ──► spawn fypd serve-api-stdio
       │
       ├──► autoConfigureMcpIfNeeded()
       │
       └──► register commands, CodeLens, watchers
       
User runs "FYP: Open Graph Editor"
       │
       ▼
openGraphPanel()
       │
       ├──► (if needed) startApiStdioServer()
       │
       └──► createWebviewPanel()
              │
              └──► Load graph-viz (React) from media/graph-viz/
                    │
                    └──► WebviewTransport connects via postMessage
```

---

## 6. Key Files Reference

| Component | Key Files |
|-----------|-----------|
| **Orchestrator (Extension)** | `build-with-me/src/extension.ts`, `build-with-me/src/ui/panels/graphPanel.ts` |
| **API bridge** | `build-with-me/src/infrastructure/services/apiStdioServer.ts` |
| **Webview transport** | `graph-viz/src/services/webviewTransport.ts` |
| **Graph API client** | `graph-viz/src/services/graphApi.ts` |
| **Backend stdio server** | `server/cmd/fypd/commands_serve_stdio.go` |
| **Backend API handlers** | `server/pkg/api/` (StdioServer, JSON-RPC routing) |
| **Binary resolution** | `build-with-me/src/utils/processHelper.ts` |

---

## 7. Graph Canvas (Draw.io-Style Flat Canvas)

The graph-viz webview renders nodes on a **flat draw.io-style canvas** — no compound/nested hierarchy. All nodes are standalone and manually positioned.

### Node Types (Cytoscape)

| `type` | Visual | Backend source |
|--------|--------|----------------|
| `directory` | Dark-green roundrectangle (border `#4a9a6a`) | Real workspace directory (synthesised from file paths) |
| `file` | Steel-blue roundrectangle (border `#7aafd4`) | Real source container/class node from indexed files |
| `method` | Blue filled circle (`#007acc`) | Real method, constructor, or field |
| `virtual-directory` | Orange dashed roundrectangle | User-created virtual directory |
| `virtual-class` | Purple dashed roundrectangle | User-created virtual class |
| `virtual` | Purple filled diamond | User-created virtual method / concept |

### Layer Z-Index

Nodes are layered so method nodes and edges always render above file/directory nodes:

| Layer | Types | Z-index |
|-------|-------|---------|
| Directory | `directory`, `virtual-directory` | 10 |
| File | `file`, `virtual-class` | 20 |
| Method + Edges | `method`, `virtual`, all edges | 30 |

Layers can be toggled on/off via the **Dir / File / Method** buttons in the canvas toolbar. Nodes can also be resized by dragging the four corner handles that appear when a node is selected.

### Interaction Model

| Action | Behaviour |
|--------|-----------|
| **Click ⊕** in file tree | Add that node to canvas; file tree `file` nodes look up the backing class node by `filePath` |
| **Single-click** canvas node | Select → show in InfoPanel |
| **Double-click** real node | Expand call-graph neighbors (methods + directed edges) |
| **Double-click** virtual node | Expand call-graph neighbors (same as real nodes) |
| **Right-click** method node | Add its parent class/file node to canvas |
| **Right-click** class/file node | Add its parent directory node to canvas |
| **Right-click** directory node | No action |
| **Drag from edge handle** | Draw an edge; saved to backend via `edges.create` |
| **Click shape in palette** | Enter placement mode (crosshair cursor) |
| **Click canvas in placement mode** | Place virtual node at that position with default label |
| **Escape** | Cancel placement mode |
| **Delete / Backspace** | Remove selected node from canvas (virtual nodes also deleted from backend) |

### Parent Lookup (Right-Click)

Parent IDs are resolved by matching raw `filePath` strings, not hash IDs:

- **Method → file**: `files.find(f => f.id === node.parentId || f.path === node.filePath)`
- **Class → directory**: `files.find(f => f.id === node.id || f.path === node.filePath)` → then `directories.find(d => d.id === fileEntry.directory)`

`parentId` on method nodes is set server-side by `computeNodeParentIDs` as `hashString(filePath)`, matching `FileNode.id`. The `filePath`-based fallback handles synthesised nodes and any case where `parentId` is absent.

### Edge Highlighting

- **Adjacent to selected node** — edges connected to the selected node turn red (`#f48771`)
- **Selected edge** — dark yellow (`#d4a017`)
- **Virtual / manual edges** — brown (`#8b5a2b`) at rest

---

## 8. MCP vs Stdio API

| Server | Spawned by | Consumers | Purpose |
|--------|------------|-----------|---------|
| **serve-api-stdio** | Extension (on activate) | Graph-viz, Index Panel, validation CodeLens | Graph data, indexing, validation |
| **serve-mcp** | Copilot (via mcp.json) | GitHub Copilot | kg.getSnapshot, kg.maskCode, kg.validateFilledCode |

These are **separate fypd processes**. The extension does not spawn the MCP server; Copilot does when using the build-with-me MCP config.

---

## Related Documentation

- [USER_FLOWS.md](USER_FLOWS.md) — User flows and sequence diagrams
- [MCP.md](MCP.md) — MCP tools and integration
- [INDEXING.md](INDEXING.md) — Indexing pipeline
- [server/API_DOCS.md](../server/API_DOCS.md) — Full stdio API reference
