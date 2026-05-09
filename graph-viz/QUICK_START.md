# Quick Start Guide

## Primary Use: Inside VS Code

The graph-viz app is designed to run as a **webview** inside the Build-With-Me VS Code extension. It communicates with the backend via postMessage (no HTTP).

### Steps

1. **Build the project** (from repo root):
   ```bash
   ./build-script/deploy.sh
   ```

2. **Open the project in VS Code** and press **F5** to launch the Extension Development Host.

3. **Open a Java workspace** and run **FYP: Build Index** to index the codebase.

4. **Run FYP: Open Graph Editor** (Command Palette or sidebar). The graph loads via the extension's stdio bridge.

5. **Interact with the graph:**
   - Click **Load All** to fetch full graph data (populates file tree)
   - Click nodes in the file tree to add them to the graph
   - Double-click a node to expand its neighbors
   - Right-click a node to remove it from the view
   - Use the Info panel to add annotations, create virtual nodes, create edges

---

## Standalone Development (Limited)

For UI development without VS Code:

```bash
cd graph-viz
npm install
npm run dev
```

Opens at http://localhost:5173/. **API calls will fail** because there is no VS Code webview to bridge requests. Use this only for styling and layout work. To test full functionality, use the Extension Development Host (F5).

---

## How to Use

### Viewing the Graph
- **Load All**: Fetches full graph; file tree populates; graph canvas stays empty until you add nodes
- **Clear**: Empties the graph canvas
- **Refresh**: Re-fetches data from backend
- **Layout**: Hierarchical, Force, or Grid

### Interacting with Nodes
- **Select**: Single click
- **Expand**: Double-click (fetches neighbors via `graph.neighborhood`)
- **Remove from view**: Right-click
- **Add to graph**: Click a method in the file tree (left panel)

### Annotations and Virtual Nodes
- Select a node → edit Description, AI Remarks, Code Snippet in the right panel
- Create virtual nodes and edges via the Info panel
- Snapshots: Use the Snapshots dropdown to create and manage snapshots

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "Failed to load graph data" | Ensure extension is running (F5) and workspace has been indexed |
| Empty graph | Click **Load All** first, then add nodes via the file tree |
| API not available | graph-viz must run inside VS Code webview; standalone dev has no backend bridge |

---

## Need Help?

See [graph-viz/README.md](README.md) and [docs/COMPONENT_ARCHITECTURE.md](../docs/COMPONENT_ARCHITECTURE.md).
