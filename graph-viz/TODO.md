# Graph Visualization App - TODO

## ✅ COMPLETED - Phase 1: Fix Node/Edge Matching

**Status:** COMPLETED ✅ (Dec 25, 2024)

### Root Cause
- Backend stored raw variable names: `mongoTemplate.count`
- Should store fully qualified names: `MongoTemplate.count`
- Edges failed to match nodes in the graph

### Solution
- Added type resolution to `server/pkg/parser/java/callgraph.go`
- Track field/variable types and resolve them when extracting method calls
- Handles field access, static calls, chained calls correctly

### To Apply
```bash
cd server && go build -o fypd ./cmd/fypd
./fypd index --workspace ~/Documents/CJON/code/CJON/  # Reindex!
./fypd serve --workspace ~/Documents/CJON/code/CJON/ --port 8080
```

**Details:** See `/Users/belvederesong/dev/learn-to-build/CALLGRAPH_FIX_SUMMARY.md`

---

## 🔧 Phase 2: Graph UI Improvements

### 1. Fix Node ID and Edge Reference Matching (BACKEND DONE - NEEDS REINDEX)

**Current Issue:**
- Nodes use hash IDs: `f87254a7a75e0249`
- Edges (method_calls) use symbol names: `ApplicationStartupListener.onApplicationEvent`
- Result: Only 1 out of 1417 edges are displayed (most are filtered out)

**What Needs to be Done:**
- [ ] **Backend: Create symbol name index**
  - Build a mapping table from symbol names to node IDs
  - Store this in the database or compute on API call
  
- [ ] **Backend: Improve edge matching**
  - Option A: Store node IDs in method_calls table instead of symbol names
  - Option B: Use fuzzy matching to handle partial symbol names
  - Option C: Create placeholder nodes for external methods
  
- [ ] **Backend: Handle external method calls**
  - Decide how to handle calls to external libraries (System.out.println, etc.)
  - Options:
    - Filter them out (current behavior)
    - Create virtual "external" nodes
    - Group by package/library
  
- [ ] **Test edge matching**
  - Verify edges connect properly
  - Should see more than 1 edge displayed

**Files to Modify:**
- `server/pkg/api/handlers_graph.go` - Edge building logic
- `server/pkg/db/graph_queries.go` - Node queries
- `server/pkg/db/callgraph.go` - Method call queries

---

### 2. Improve Graph UI Layout

**Current State:**
- Basic 3-panel layout with collapsible sides ✅
- Empty graph by default ✅
- Header with actions ✅

**What Needs to be Done:**

#### Layout & Panels
- [ ] **Test and verify panel behavior**
  - Ensure left/right panels work independently
  - Verify graph resizes correctly when panels collapse
  - Check that header stays on top
  
- [ ] **Improve panel UI**
  - Better collapse/expand buttons (more intuitive icons)
  - Add panel width indicators
  - Smooth animations when collapsing
  
- [ ] **File Tree improvements**
  - Add loading state when fetching data
  - Show file/folder counts
  - Better icons for different file types
  - Keyboard navigation support

#### Graph Canvas
- [ ] **Visual improvements**
  - Better node colors by type (class/method/field)
  - Edge thickness based on call count
  - Highlight on hover
  - Selected node highlight
  
- [ ] **Layout options**
  - Make hierarchical layout default ✅
  - Fine-tune dagre settings (spacing, direction)
  - Test force and grid layouts
  
- [ ] **Interaction improvements**
  - Add node/edge labels on hover
  - Context menu on right-click (instead of just remove)
  - Drag to pan canvas
  - Scroll to zoom
  
- [ ] **Mini-map**
  - Add small overview map in corner
  - Shows full graph with viewport indicator
  
- [ ] **Auto-center on load**
  - Currently implemented ✅
  - Test with various graph sizes

#### Info Panel
- [ ] **Better empty state**
  - Show instructions: "Click a node in the file tree to add it to the graph"
  - Show keyboard shortcuts
  
- [ ] **Node details improvements**
  - Show method signature
  - Link to file location
  - Show incoming/outgoing edges count
  - Related nodes list
  
- [ ] **Annotation features**
  - Rich text editor for descriptions
  - Tags/categories
  - Search annotations

#### Header
- [ ] **Better action buttons**
  - Icons instead of text
  - Tooltips on hover
  - Loading states
  
- [ ] **Add more controls**
  - Filter by node type (class/method/field)
  - Filter by package
  - Search across all nodes
  
- [ ] **Graph stats**
  - Show more detailed stats
  - Node type breakdown
  - Edge count by type

---

## 🎨 Nice-to-Have Features

### Graph Features
- [ ] **Node grouping**
  - Group nodes by file/package
  - Collapsible groups
  
- [ ] **Path finding**
  - Find path between two nodes
  - Highlight path in graph
  
- [ ] **Export**
  - Export graph as PNG/SVG
  - Export as JSON
  
- [ ] **Save/Load views**
  - Save current graph state
  - Load saved views
  - Share views with URL

### Performance
- [ ] **Virtualization**
  - Only render visible nodes in large graphs
  - Lazy load node details
  
- [ ] **Caching**
  - Cache API responses
  - Remember panel states in localStorage

### Integration
- [ ] **Electron wrapper** (Phase 3)
  - Package as desktop app
  - File system access
  
- [ ] **VS Code extension** (Phase 4)
  - Integrate into VS Code
  - Jump to code from graph

---

## 🐛 Known Issues

1. **Only 1 edge displayed** (see #1 above)
   - Backend filters out edges with missing nodes
   - Need better symbol matching

2. **Empty graph on first load**
   - By design, but could show a welcome message
   - Or load a sample/popular nodes

3. **File tree might be slow with large codebases**
   - Consider pagination or lazy loading
   - Virtual scrolling for long lists

---

## 📝 Current Progress

### ✅ Completed
- Backend API server with all endpoints
- React frontend with 3-panel layout
- Basic graph visualization with Cytoscape.js
- Node/edge selection and details
- Collapsible side panels
- Empty graph by default (add nodes on demand)
- Auto-center graph on nodes
- File tree browser
- Info panel with annotations
- CRUD operations for annotations

### 🔄 In Progress
- Layout improvements
- Edge matching fixes

### ⏳ Not Started
- Electron wrapper
- VS Code extension

---

## 🚀 Quick Start (for next session)

1. **Start backend:**
   ```bash
   cd server
   ./fypd serve --workspace ~/Documents/CJON/code/CJON/ --port 8080
   ```

2. **Start frontend:**
   ```bash
   cd graph-viz
   npm run dev
   ```

3. **Open browser:**
   ```
   http://localhost:5173/
   ```

4. **Test edge matching:**
   - Click "Load All" button
   - Check console for: "Graph stats: X nodes, Y edges (skipped Z edges)"
   - Goal: Reduce skipped edges count

---

## 📚 Resources

- **Cytoscape.js Docs:** https://js.cytoscape.org/
- **React Resizable Panels:** https://github.com/bvaughn/react-resizable-panels
- **Current Plan:** `.cursor/plans/call_graph_visualization_app_5a7ab6ec.plan.md`
- **API Docs:** `server/API_DOCS.md`

---

## 💡 Ideas for Future

- AI-powered graph analysis
  - Suggest refactoring opportunities
  - Identify code smells
  - Detect circular dependencies
  
- Collaborative features
  - Share annotations with team
  - Real-time updates
  
- Integration with other tools
  - Jira/GitHub issues
  - Code review comments
  - Documentation links

