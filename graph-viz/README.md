# Graph Knowledge Editor

An interactive graph-based knowledge management system for codebase visualization, planning, and annotation.

## Purpose

The graph editor addresses the **verification overhead** in agentic AI pair programming by providing visual understanding of how code connects.

| Problem | How Graph Editor Helps |
|---------|------------------------|
| Hard to verify AI code integration | See method relationships at a glance |
| No architectural visibility | Interactive call graph exploration |
| AI ignores project conventions | Annotations document patterns for AI |
| Planning is disconnected | Virtual nodes scope future features |

---

## Features

- **3-Panel Interface**
  - Left: File tree showing workspace structure
  - Center: Interactive Cytoscape.js graph canvas
  - Right: Info/edit panel with node details and annotations

- **Interactive Graph**
  - Click to select nodes/edges
  - Double-click to expand node neighbors
  - Right-click to remove nodes from view
  - Drag to reposition nodes
  - Multiple layout algorithms (Hierarchical, Force-Directed, Grid)

- **Annotations**
  - Add descriptions to nodes and edges
  - AI-specific remarks for planning
  - Code snippets for virtual nodes

- **Virtual Nodes**
  - Create planning nodes not tied to actual code
  - Distinguish virtual from real code nodes

## Architecture

This app is designed to run as a **VS Code webview** inside the Build-With-Me extension. It communicates with the backend via:

1. **VS Code postMessage** - Webview sends requests to the extension
2. **Extension stdio bridge** - Extension forwards requests to `fypd serve-api-stdio`
3. **JSON-RPC over stdio** - Backend processes requests and returns results

```
┌─────────────────────────────────────────────────────────┐
│  VS Code Webview (graph-viz)                            │
│  React + Cytoscape.js                                   │
└───────────────────────────┬─────────────────────────────┘
                            │ postMessage
                            ▼
┌─────────────────────────────────────────────────────────┐
│  VS Code Extension                                      │
│  (apiStdioServer.ts)                                    │
└───────────────────────────┬─────────────────────────────┘
                            │ JSON-RPC over stdio
                            ▼
┌─────────────────────────────────────────────────────────┐
│  fypd serve-api-stdio                                   │
│  (Go backend)                                           │
└─────────────────────────────────────────────────────────┘
```

## Development

### Prerequisites

- Node.js 20.19+ or 22.12+
- pnpm (recommended) or npm

### Installation

```bash
npm install
```

### Start Development Server

For standalone development (without VS Code):

```bash
npm run dev
```

The app will be available at http://localhost:5173/

**Note:** In standalone mode, API calls will fail unless you modify `webviewTransport.ts` to use a mock or connect to a running backend.

### Build for Production

```bash
npm run build
```

The built files will be in the `dist/` directory. These are copied to `build-with-me/media/graph-viz/` for the VS Code extension.

## Project Structure

```
graph-viz/
├── src/
│   ├── components/
│   │   ├── FileTree/       # File tree panel
│   │   ├── GraphCanvas/    # Cytoscape.js graph
│   │   ├── InfoPanel/      # Node/edge details
│   │   └── Layout/         # 3-panel layout
│   ├── hooks/              # Custom React hooks
│   ├── services/
│   │   ├── graphApi.ts     # API client
│   │   └── webviewTransport.ts  # postMessage transport
│   ├── types/              # TypeScript types
│   ├── utils/              # Helper functions
│   ├── App.tsx             # Main app component
│   └── main.tsx            # Entry point
├── package.json
├── tsconfig.json
└── vite.config.ts
```

## Technologies

- **React** - UI framework
- **TypeScript** - Type safety
- **Vite** - Build tool
- **Cytoscape.js** - Graph visualization
- **react-resizable-panels** - Resizable layout

---

## Implementation Details

### Component Architecture

```
src/
├── App.tsx                    # Main layout with 3 panels
├── components/
│   ├── FileTree/              # Left panel - workspace navigation
│   │   └── FileTree.tsx       # Recursive tree with file icons
│   ├── GraphCanvas/           # Center panel - Cytoscape.js graph
│   │   └── GraphCanvas.tsx    # Graph rendering, layout selector, zoom controls
│   ├── InfoPanel/             # Right panel - node/edge details
│   │   └── InfoPanel.tsx      # Selected item, annotations, create node/edge forms
│   └── SnapshotsDropdown/     # Snapshot list and capture UI
├── hooks/
│   ├── useGraph.ts            # Graph state management
│   ├── useSelection.ts        # Selected node/edge state
│   └── useFileTree.ts         # File tree data fetching
├── services/
│   ├── graphApi.ts            # API client (JSON-RPC calls)
│   └── webviewTransport.ts    # VS Code postMessage bridge
└── utils/
    ├── cytoscapeStyles.ts     # Node/edge styling
    └── layoutConfig.ts        # Layout algorithm configs
```

### State Management (`useGraph.ts`)

`useGraph` manages the full graph data (`graphData`) from the API. The display graph (`displayGraphData`) is managed by `App.tsx` and starts empty; users add nodes via the file tree or double-click to expand. Key operations:

- `loadGraph()` — fetches full graph via `graph.get`
- `expandNode(nodeId)` — fetches neighborhood via `graph.neighborhood`, merges into `graphData`
- `createVirtualNode`, `createEdge`, `deleteVirtualNode`, `deleteEdge` — CRUD via API

### VS Code Integration (`webviewTransport.ts`)

When running in a webview, `webviewTransport` uses `acquireVsCodeApi().postMessage()` to send `apiRequest` messages to the extension. The extension forwards them to `fypd serve-api-stdio` via stdin and posts `apiResponse` back. No HTTP; all communication is message-passing through the extension.

### Cytoscape.js Graph (`GraphCanvas.tsx`)

Renders nodes and edges from `graphBuilder.buildCytoscapeElements()`. Supports tap (select), dbltap (expand neighbors), cxttap (remove from view). Layout options: hierarchical (dagre), force-directed, grid. Nodes are nested under file/directory parents for structure.

---

## API Methods

The webview communicates via JSON-RPC methods:

| Method | Description |
|--------|-------------|
| `graph.get` | Get full graph data |
| `graph.neighborhood` | Get node neighbors |
| `graph.search` | Search nodes |
| `filetree.get` | Get file tree |
| `nodes.list/get/create/update/delete` | Node operations |
| `edges.list/create/delete` | Edge operations |
| `annotations.node.get/update` | Node annotations |
| `annotations.edge.get/update` | Edge annotations |
| `snapshots.list/create/get/delete` | Snapshot operations |

## Usage (in VS Code)

1. Open the Build-With-Me extension
2. Click "Open Graph Editor" in the sidebar
3. The graph loads automatically from the indexed workspace
4. Interact with the graph:
   - Click nodes to view details
   - Double-click to expand neighbors
   - Right-click to remove from view
   - Use controls to zoom and change layouts
5. Edit annotations in the right panel
6. Browse the file tree in the left panel

## Current Capabilities

- Click file tree to add nodes to graph
- Double-click node to expand neighbors
- Create virtual nodes and edges via InfoPanel
- Create and manage snapshots via SnapshotsDropdown
- `graph.search` API available for node search

---

## Related Documentation

- [VISION.md](../VISION.md) - Problem statement and feature overview
- [MASKING.md](../MASKING.md) - Masking system details
- [VALIDATION.md](../VALIDATION.md) - Validation pipeline details
- [build-with-me/README.md](../build-with-me/README.md) - VS Code extension details
- [server/API_DOCS.md](../server/API_DOCS.md) - Backend API reference
