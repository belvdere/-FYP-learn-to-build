# Codebase Graph Visualization and Snapshot Implementation

This document describes the implementation details of the codebase graph visualization system and snapshot functionality.

## Table of Contents

1. [Graph Data Source: AST-Based Indexing](#graph-data-source-ast-based-indexing)
2. [Backend API Endpoints](#backend-api-endpoints)
3. [Frontend: React and Cytoscape.js](#frontend-react-and-cytoscapejs)
4. [Interactive Features](#interactive-features)
5. [Snapshot System](#snapshot-system)

---

## Graph Data Source: AST-Based Indexing

The graph visualization is built from **Abstract Syntax Tree (AST)** data extracted from source code. This approach provides structural accuracy and precise source mapping.

### Data Extraction

The indexing pipeline (see `INDEXING.md` for full details) extracts:

- **Symbols**: Classes, methods, constructors, and fields from AST nodes
  - Format: `ClassName.methodName`, `ClassName.FieldName`, `ClassName.<init>` (constructors)
  - Stored in the `symbols` table with file path and line number

- **Method Calls**: Directed edges representing method invocations
  - Format: `callerSymbol → calleeSymbol`
  - Captures direct calls, static calls, and constructor calls (`new ClassName()` → `ClassName.<init>`)
  - Stored in the `method_calls` table with call type, file path, and line number

- **Virtual Nodes**: User-created placeholder nodes for planned code
  - Stored in the `virtual_nodes` table with descriptions and AI remarks

### Graph Node Types

The backend aggregates symbols and virtual nodes into unified graph nodes:

- **Real nodes**: Extracted from AST (`symbols` table)
- **Virtual nodes**: User-created placeholders (`virtual_nodes` table)
- **File nodes**: Compound nodes grouping methods within a file
- **Directory nodes**: Parent nodes containing files

### Edge Aggregation

The backend aggregates method calls into directed edges:

- Multiple call sites between the same symbols are counted (`count` field)
- Edges are directed: `source → target` (caller → callee)
- Edge IDs use format: `sourceID::targetID`
- Manual edges (user-created) are flagged with `isManual: true`

---

## Backend API Endpoints

The backend provides JSON-RPC endpoints over stdio (`fypd serve-api-stdio`). Major endpoints:

### Graph Query Endpoints

#### `getGraph`
Returns the complete graph with all nodes and edges.

**Logic:**
1. Fetches all graph nodes (real + virtual) from database
2. Fetches all method calls
3. Builds symbol-to-node-ID mapping
4. Aggregates method calls into edges:
   - Groups by `sourceID::targetID`
   - Counts occurrences
   - Flags manual edges (`call_type = "manual"`)
   - Attaches edge annotations if present
5. Builds file/directory hierarchy from node file paths
6. Returns `GraphData` with nodes, edges, files, and directories

#### `getNeighborhood`
Returns a subgraph containing a node and its immediate neighbors.

**Logic:**
1. Fetches the target node by ID
2. Queries all method calls where the node appears as caller or callee
3. Identifies neighbor nodes (the other endpoint of each edge)
4. Aggregates edges connected to the target node
5. Returns only the target node, its neighbors, and connecting edges

#### `searchNodes`
Searches for nodes by label (symbol name).

**Logic:**
- Performs SQL `LIKE` query on node labels
- Returns up to 50 matching nodes
- Used for node search/autocomplete in the UI

### Edge Management Endpoints

#### `createEdge`
Creates a manual edge between two nodes.

**Logic:**
- Validates source and target nodes exist
- Creates a `MethodCall` record with:
  - `call_type: "manual"`
  - `file_path: ""` (empty indicates user-created)
  - `line: 0`
- Returns the created edge with `isManual: true`

#### `deleteEdge`
Deletes a manual edge (only edges with `file_path = ""` can be deleted).

**Logic:**
- Deletes `MethodCall` records matching the edge ID where `file_path = ""`
- Preserves AST-extracted edges (non-empty `file_path`)

#### `listEdges`
Lists all edges, aggregated by source/target pairs.

**Logic:**
- Similar aggregation to `getGraph` but returns only edges
- Includes annotation data if present

### Virtual Node Endpoints

#### `createVirtualNode`
Creates a user-defined placeholder node.

**Logic:**
- Inserts into `virtual_nodes` table
- Supports optional file/directory placement
- Returns the created node with `isVirtual: true`

#### `deleteVirtualNode`
Deletes a virtual node and its manual edges.

**Logic:**
- Deletes the node from `virtual_nodes`
- Deletes all manual edges connected to it

---

## Frontend: React and Cytoscape.js

The frontend is built with **React** and uses **Cytoscape.js** for graph rendering.

### Technology Stack

- **React**: Component-based UI framework
- **Cytoscape.js** (`^3.33.1`): Graph visualization library
- **cytoscape-dagre** (`^2.5.0`): Hierarchical layout algorithm (Dagre)
- **TypeScript**: Type-safe development

### Component Architecture

```
App.tsx
├── ThreePanelLayout
│   ├── FileTreePanel (left)
│   ├── GraphCanvas (center)
│   └── InfoPanel (right)
└── SnapshotsDropdown (overlay)
```

### Graph Rendering Pipeline

1. **Data Fetching**: `useGraph` hook fetches `GraphData` from backend
2. **Transformation**: `buildCytoscapeElements()` converts API data to Cytoscape format:
   - Creates directory nodes (parent containers)
   - Creates file nodes (compound nodes, children of directories)
   - Creates method/virtual nodes (children of files)
   - Creates directed edges with styling flags
   - Formats node labels (splits on last dot for readability)
3. **Rendering**: Cytoscape instance renders elements with styles
4. **Layout**: Dagre layout arranges nodes hierarchically

### Cytoscape Configuration

**Initialization:**
- Single Cytoscape instance created once (prevents viewport reset)
- Callbacks stored in refs to avoid instance recreation
- Styles applied via `cytoscapeStyles` configuration

**Styling:**
- VS Code Dark+ theme colors
- Node types: directory (gray), file (dark blue), method (blue), virtual (purple diamond)
- Edge types: regular (gray), virtual/user-created (red)
- Selection/hover effects with overlays

**Layouts:**
- **Dagre** (default): Hierarchical top-to-bottom layout
- **COSE**: Force-directed physics-based layout
- **Grid**: Regular grid arrangement
- Layouts are animated (500ms duration)

### Design Considerations

#### Performance
- **Single Cytoscape instance**: Prevents viewport reset on re-renders
- **Batch updates**: Elements added/removed in batches
- **Callback refs**: Avoids recreating Cytoscape on callback changes
- **Deduplication**: `mergeGraphData()` prevents duplicate nodes/edges

#### User Experience
- **Incremental graph building**: Users start with empty graph, add nodes via file tree
- **Neighborhood expansion**: Double-click node to expand neighbors
- **Compound nodes**: Files contain methods, directories contain files (visual hierarchy)
- **Zoom/pan controls**: Zoom in/out, fit to screen, reset view
- **Layout switching**: Users can switch between layout algorithms

#### State Management
- **Local state**: Graph data stored in React state (`useGraph` hook)
- **Optimistic updates**: Virtual nodes/edges added immediately, synced with backend
- **Merging strategy**: New data merged into existing graph (no full refresh)

---

## Interactive Features

### File Tree Panel

- **Browse codebase**: Hierarchical file/directory tree
- **Click to add**: Click a file/method to add it to the graph
- **Search**: Filter tree by name
- **Context menu**: Right-click to create virtual nodes/directories

### Graph Canvas

- **Node interactions**:
  - **Click**: Select node (shows details in Info Panel)
  - **Double-click**: Expand neighbors (fetches and adds connected nodes)
  - **Right-click**: Context menu (future: quick actions)
  - **Drag**: Pan the graph

- **Edge interactions**:
  - **Click**: Select edge (shows details in Info Panel)
  - **Hover**: Highlight edge

- **View controls**:
  - **Zoom**: Mouse wheel or zoom buttons
  - **Pan**: Click and drag background
  - **Layout**: Switch between Dagre, COSE, Grid
  - **Fit to screen**: Auto-zoom to show all nodes
  - **Reset view**: Return to default zoom/pan

### Info Panel

- **Node details**: Shows label, type, file path, line number, annotations
- **Edge details**: Shows source/target, call count, annotations
- **Virtual node creation**: Form to create placeholder nodes
- **Edge creation**: Connect two nodes manually
- **Deletion**: Delete virtual nodes and manual edges
- **Snapshot creation**: Capture current graph state

### Graph Building Workflow

1. **Load graph data**: Click "Load All" to populate file tree (graph stays empty)
2. **Add nodes**: Click files/methods in tree to add to graph
3. **Expand**: Double-click nodes to add neighbors
4. **Search**: Use search to find and add specific nodes
5. **Create virtual nodes**: Add placeholder nodes for planned code
6. **Create edges**: Manually connect nodes
7. **Annotate**: Add descriptions/remarks to nodes and edges

---

## Snapshot System

Snapshots capture the current graph state for later use in code generation workflows.

### Snapshot Data Model

A snapshot stores:

- **Virtual Nodes**: User-created virtual nodes to generate (for example virtual directory/class/method), including hierarchy (`parentId`) and optional node notes (`description`, `aiRemarks`)
- **Context Nodes**: Existing indexed nodes kept as implementation reference, using the same node payload shape
- **Edges**: Selected relationships between virtual/context nodes, including optional edge remarks
- **Name**: Optional user-provided label
- **Custom Prompt**: Optional user-edited prompt override (falls back to generated markdown when unset)

Persistence details:

- Snapshot payload is stored in the `snapshots` table with JSON columns: `virtual_nodes`, `context_nodes`, and `edges`
- Metadata is stored in scalar columns (`id`, `name`, timestamps, and `custom_prompt`)
- JSON storage keeps the schema flexible as snapshot payload fields evolve

### Snapshot Creation

**Frontend Flow (`InfoPanel`):**

1. User selects nodes/edges in the graph
2. Clicks "Capture Snapshot" button
3. Frontend categorizes nodes:
   - **Virtual nodes**: Nodes with `isVirtual: true`
   - **Context nodes**: Real nodes (non-virtual)
4. Collects all edges between selected nodes
5. Sends `createSnapshot` API request with:
   - `virtualNodes`: Array of virtual node data
   - `contextNodes`: Array of context node data
   - `edges`: Array of edge data
   - `name`: Optional snapshot name

**Backend Logic (`handleCreateSnapshot`):**

1. Parses virtual nodes, context nodes, and edges from request
2. Generates unique snapshot ID (`snap_<uuid>`)
3. Stores snapshot in `snapshots` table (JSON columns)
4. Returns snapshot metadata (ID, name, counts, timestamp)

### Snapshot Loading

Loading re-materializes a saved snapshot into the current graph state.

**Frontend Flow (`App.handleLoadSnapshot`):**

1. Clears current virtual state (`graph.clearVirtual`) and UI selection
2. Calls `snapshots.load` to restore snapshot state into DB
3. Reloads fresh graph data (`graph.get`) and fetches snapshot detail (`snapshots.get`)
4. Builds display graph via `buildSnapshotDisplayData(...)` so stale snapshot references are still visible as read-only placeholders
5. Shows an info message when snapshot context nodes are stale (`missingContextNodes > 0`)

**Backend Logic (`handleLoadSnapshot` + `LoadSnapshotState`):**

1. Reads snapshot by ID from `snapshots` table
2. Restores virtual nodes, relevant annotations, and edges in a single transaction
3. Preserves real-real relationships by materializing manual edges when indexed edges are absent
4. Computes and returns `restoredVirtualNodes`, `restoredEdges`, and `missingContextNodes`

### Snapshot Prompt Generation

Snapshots are converted to markdown prompts for LLM consumption.

**Auto-Generated Prompt (`FormatSnapshotAsMarkdown`):**

The backend generates a structured markdown prompt:

```markdown
# Graph Snapshot: <id>

## Virtual Nodes (To Generate)
[List of virtual nodes with descriptions]

## Context Nodes (Existing Code)
[List of context nodes with file paths]

## Relationships
[List of edges showing connections]

## Generation Instructions
[Instructions for implementing virtual nodes]
```

**Custom Prompt Editing:**

Users can edit the snapshot prompt:

1. **View prompt**: Open snapshot from dropdown → "Preview Prompt"
2. **Edit**: Modify the prompt text in the modal
3. **Save**: Stores custom prompt in `snapshots.custom_prompt` column
4. **Reset**: Revert to auto-generated prompt (clears `custom_prompt`)

**Backend Endpoints:**

- `snapshots.prompt.get`: Returns prompt (custom if set, else generated)
- `snapshots.prompt.update`: Saves custom prompt
- `snapshots.prompt.reset`: Clears custom prompt

### Design Considerations

#### Prompt Flexibility
- **Auto-generation**: Provides consistent, structured format
- **Custom editing**: Allows users to refine prompts for specific needs
- **Reset option**: Easy return to auto-generated version

#### Snapshot Storage
- **JSON columns**: Flexible schema for node/edge data
- **Metadata**: Name, timestamps, custom prompt stored separately
- **Query efficiency**: Snapshots loaded on-demand (not in graph queries)

#### User Workflow
- **Visual selection**: Users select nodes/edges in graph (intuitive)
- **Categorization**: Automatic separation of virtual vs. context nodes
- **Preview before use**: Users can review/edit prompts before using in workflows

---

## Summary

The graph visualization system combines:

- **AST-based indexing** for accurate code structure extraction
- **Efficient backend aggregation** for scalable graph queries
- **Cytoscape.js rendering** for interactive visualization
- **Incremental graph building** for responsive user experience
- **Snapshot system** for capturing and reusing graph states

The architecture prioritizes performance (single Cytoscape instance, batch updates), user experience (incremental building, multiple layouts), and flexibility (virtual nodes, manual edges, custom prompts).
