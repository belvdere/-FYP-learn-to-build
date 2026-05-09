/**
 * Main App component - Graph visualization for method call relationships.
 * Three-panel layout: FileTree (left), GraphCanvas (center), InfoPanel (right).
 * Incremental canvas with conditional parent-child containment.
 */

import { useState, useCallback, useEffect, useRef, useMemo } from 'react';
import { ThreePanelLayout } from './components/Layout/ThreePanelLayout';
import { FileTreePanel } from './components/FileTree/FileTreePanel';
import { GraphCanvas } from './components/GraphCanvas/GraphCanvas';
import { ShapePalette, PlacementTool } from './components/GraphCanvas/ShapePalette';
import { InfoPanel } from './components/InfoPanel/InfoPanel';
import { SnapshotsDropdown } from './components/SnapshotsDropdown';
import { useGraph } from './hooks/useGraph';
import { useFileTree } from './hooks/useFileTree';
import { useSelection } from './hooks/useSelection';
import { TreeNode } from './types/fileTree';
import { GraphNode, GraphEdge, GraphData } from './types/graph';
import {
  mergeGraphData,
  findConnectedEdges,
  fileNodeToGraphNode,
  dirNodeToGraphNode,
} from './utils/graphBuilder';
import { buildSnapshotDisplayData } from './utils/snapshotDisplay';
import { graphApi } from './services/graphApi';
import { postMessage } from './services/webviewTransport';
import './App.css';

const EMPTY_GRAPH_DATA: GraphData = {
  nodes: [],
  edges: [],
  files: [],
  directories: [],
};

function App() {
  const {
    graphData,
    loading,
    error,
    loadGraph,
    expandNode,
    refresh,
    getGraphData,
    deleteVirtualNode,
    deleteEdge,
    addNodeToGraphData,
    addEdgeToGraphData,
    removeNodeFromGraphData,
  } = useGraph();

  const [displayGraphData, setDisplayGraphData] = useState<GraphData>(EMPTY_GRAPH_DATA);
  const [placementMode, setPlacementMode] = useState<PlacementTool | null>(null);

  // Pending positions for nodes placed via palette: nodeId → graph coords
  // GraphCanvas reads and clears entries after positioning the Cytoscape node.
  const pendingNodePositionsRef = useRef<Map<string, { x: number; y: number }>>(new Map());

  const { treeData, expandedNodes, toggleNode } = useFileTree(graphData);
  const { selection, selectNode, selectEdge, clearSelection } = useSelection();

  const [showSnapshotsDropdown, setShowSnapshotsDropdown] = useState(false);
  const connectableNodeIds = useMemo(
    () => new Set(graphData.nodes.map(node => node.id)),
    [graphData.nodes]
  );

  const findVisibleDirectoryByPath = useCallback((dirPath: string): GraphNode | undefined => {
    return displayGraphData.nodes.find(
      n => n.type === 'directory' && !n.isVirtual && n.filePath === dirPath
    );
  }, [displayGraphData.nodes]);

  // Escape key cancels placement mode
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setPlacementMode(null);
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  const handleExternalRefresh = useCallback(async () => {
    const updatedData = await refresh();
    setDisplayGraphData(prev => {
      if (prev.nodes.length === 0 && prev.edges.length === 0) {
        return prev;
      }

      const previousVisibleIDs = new Set(prev.nodes.map(n => n.id));
      const refreshedNodes = updatedData.nodes.filter(n => previousVisibleIDs.has(n.id));
      const refreshedNodeIDs = new Set(refreshedNodes.map(n => n.id));

      const syntheticNodes = prev.nodes.filter(n =>
        !refreshedNodeIDs.has(n.id) && (n.type === 'directory' || n.type === 'file')
      );
      const finalNodeIDs = new Set([
        ...refreshedNodeIDs,
        ...syntheticNodes.map(n => n.id),
      ]);

      const refreshedEdges = updatedData.edges.filter(e =>
        finalNodeIDs.has(e.source) && finalNodeIDs.has(e.target)
      );

      return {
        ...prev,
        files: updatedData.files,
        directories: updatedData.directories,
        nodes: [...refreshedNodes, ...syntheticNodes],
        edges: refreshedEdges,
      };
    });
  }, [refresh]);

  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      const message = event.data as { type?: string } | undefined;
      if (message?.type === 'graphRefreshRequested') {
        void handleExternalRefresh();
      }
    };
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [handleExternalRefresh]);

  const handleLoadAll = useCallback(async () => {
    await loadGraph();
  }, [loadGraph]);

  const cleanupAllVirtualState = useCallback(async () => {
    await graphApi.clearVirtualState();
    setDisplayGraphData(EMPTY_GRAPH_DATA);
    clearSelection();
    await refresh();
  }, [clearSelection, refresh]);

  const handleClear = useCallback(async () => {
    await cleanupAllVirtualState();
  }, [cleanupAllVirtualState]);

  const handleLoadSnapshot = useCallback(async (id: string) => {
    await graphApi.clearVirtualState();
    setDisplayGraphData(EMPTY_GRAPH_DATA);
    clearSelection();
    const result = await graphApi.loadSnapshot(id);
    const freshData = await loadGraph();
    const snapshotDetail = await graphApi.getSnapshot(id);
    const displayData = buildSnapshotDisplayData(freshData, snapshotDetail);
    setDisplayGraphData(displayData);
    if (result.missingContextNodes > 0) {
      postMessage({ type: 'showInfo', text: `Loaded snapshot: ${result.missingContextNodes} context node(s) are stale (no longer in codebase)` });
    }
  }, [clearSelection, loadGraph]);

  // Node click in graph canvas — select for InfoPanel
  const handleNodeClick = useCallback((node: GraphNode) => {
    selectNode(node);
  }, [selectNode]);

  // Double-click: expand neighborhood for both real and virtual nodes
  const handleNodeDoubleClick = useCallback(async (node: GraphNode) => {
    // Expand: fetch neighborhood and add neighbors to display.
    // Read displayGraphData via functional updater to capture the latest snapshot
    // even if this async callback was created with a stale closure.
    const expandedData = await expandNode(node.id);

    setDisplayGraphData(prev => {
      const displayNodeIds = new Set(prev.nodes.map(n => n.id));
      displayNodeIds.add(node.id);

      const connectedEdges = expandedData.edges.filter(
        edge => edge.source === node.id || edge.target === node.id
      );

      const neighborNodeIds = new Set<string>();
      connectedEdges.forEach(edge => {
        if (edge.source === node.id) neighborNodeIds.add(edge.target);
        else if (edge.target === node.id) neighborNodeIds.add(edge.source);
      });

      const neighborNodes = expandedData.nodes.filter(
        n => neighborNodeIds.has(n.id) && !displayNodeIds.has(n.id)
      );
      neighborNodes.forEach(n => displayNodeIds.add(n.id));

      const validEdges = connectedEdges.filter(
        edge => displayNodeIds.has(edge.source) && displayNodeIds.has(edge.target)
      );

      return mergeGraphData(prev, {
        nodes: neighborNodes,
        edges: validEdges,
        files: expandedData.files,
        directories: expandedData.directories,
      });
    });
  }, [expandNode]);

  // Right-click node: add its parent to canvas
  const handleNodeRightClickAddParent = useCallback((node: GraphNode) => {
    const currentData = getGraphData();

    if (node.type === 'method' || node.type === 'field') {
      // Prefer the real class node from graph data (proper label, no .java extension)
      // over synthesizing a file node from fileNodeToGraphNode
      let parentNode: GraphNode | undefined = currentData.nodes.find(
        n => n.filePath === node.filePath && (n.type === 'class' || n.type === 'interface') && !n.isVirtual
      );
      if (!parentNode) {
        const fileEntry = currentData.files.find(
          f => f.id === node.parentId || f.path === node.filePath
        );
        if (fileEntry) parentNode = fileNodeToGraphNode(fileEntry);
      }
      if (parentNode && !displayGraphData.nodes.some(n => n.id === parentNode!.id)) {
        setDisplayGraphData(prev => mergeGraphData(prev, { nodes: [parentNode!], edges: [] }));
      }
    } else if (node.type === 'class' || node.type === 'interface') {
      // Real class nodes: node.id = DB symbol ID (≠ f.id); use f.path === node.filePath instead
      // Synthesised class nodes (from fileNodeToGraphNode): node.id = hashString(filePath) = f.id
      const fileEntry = currentData.files.find(
        f => f.id === node.id || f.path === node.filePath
      );
      if (fileEntry) {
        const dirEntry = currentData.directories.find(d => d.id === fileEntry.directory);
        if (dirEntry && !displayGraphData.nodes.some(n => n.id === dirEntry.id) && !findVisibleDirectoryByPath(dirEntry.path)) {
          const dirGraphNode = dirNodeToGraphNode(dirEntry);
          setDisplayGraphData(prev => mergeGraphData(prev, { nodes: [dirGraphNode], edges: [] }));
        }
      }
    }
    // directory: no action
  }, [getGraphData, displayGraphData.nodes, findVisibleDirectoryByPath]);

  // Place a virtual node at the clicked canvas position
  const handlePlaceVirtualNode = useCallback(async (type: PlacementTool, position: { x: number; y: number }) => {
    const defaultLabels: Record<PlacementTool, string> = {
      'virtual-directory': 'New Virtual Directory',
      'virtual-class': 'New Virtual Class',
      'virtual': 'New Virtual Method',
    };
    const apiType = type === 'virtual' ? 'method' : type === 'virtual-class' ? 'class' : 'directory';

    try {
      const created = await graphApi.createVirtualNode({ label: defaultLabels[type], type: apiType });
      if (created) {
        // Register the intended position before adding to display; GraphCanvas will apply it.
        pendingNodePositionsRef.current.set(created.id, position);
        addNodeToGraphData(created);
        setDisplayGraphData(prev => mergeGraphData(prev, { nodes: [created], edges: [] }));
        selectNode(created);
        setPlacementMode(null);
      }
    } catch (err) {
      postMessage({ type: 'showError', text: 'Failed to create virtual node' });
    }
  }, [addNodeToGraphData, selectNode]);

  // Edge drawn via edge handles — create via API
  const handleEdgeDrawn = useCallback(async (sourceId: string, targetId: string) => {
    if (!connectableNodeIds.has(sourceId) || !connectableNodeIds.has(targetId)) {
      postMessage({
        type: 'showError',
        text: 'Edge draw is only supported between indexed/virtual graph nodes (not file/directory placeholders)',
      });
      return;
    }
    if (displayGraphData.edges.some(e => e.source === sourceId && e.target === targetId)) return;
    try {
      const edge = await graphApi.createEdge({ sourceNodeId: sourceId, targetNodeId: targetId });
      if (edge) {
        addEdgeToGraphData(edge);
        setDisplayGraphData(prev => mergeGraphData(prev, { nodes: [], edges: [edge] }));
        selectEdge(edge);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      postMessage({ type: 'showError', text: `Failed to create edge: ${message}` });
    }
  }, [connectableNodeIds, displayGraphData.edges, addEdgeToGraphData, selectEdge]);

  // Edge click — select edge
  const handleEdgeClick = useCallback((edge: GraphEdge) => {
    selectEdge(edge);
  }, [selectEdge]);

  // Background click — clear selection
  const handleBackgroundClick = useCallback(() => {
    clearSelection();
  }, [clearSelection]);

  // Tree node click — just select for InfoPanel (no canvas add)
  const handleTreeNodeSelect = useCallback(async (node: TreeNode) => {
    let currentData = getGraphData();
    if (currentData.nodes.length === 0) {
      currentData = await loadGraph();
    }
    let graphNode = currentData.nodes.find(n => n.id === node.id);
    if (!graphNode && node.type === 'file') {
      graphNode = currentData.nodes.find(n =>
        n.filePath === node.path && (n.type === 'file' || n.type === 'class' || n.type === 'interface') && !n.isVirtual
      );
      if (!graphNode) {
        const fileEntry = currentData.files.find(f => f.path === node.path);
        if (fileEntry) graphNode = fileNodeToGraphNode(fileEntry);
      }
    }
    if (graphNode) selectNode(graphNode);
  }, [getGraphData, loadGraph, selectNode]);

  // Tree ⊕ button — add node to canvas
  const handleAddToCanvas = useCallback(async (node: TreeNode) => {
    let currentData = getGraphData();
    if (currentData.nodes.length === 0) {
      currentData = await loadGraph();
    }

    // Method nodes have real IDs; file/directory tree nodes use synthetic IDs (file:..., dir:...)
    // so we fall back to filePath-based lookup for those.
    let graphNode = currentData.nodes.find(n => n.id === node.id);

    if (!graphNode && node.type === 'file') {
      // Find the class/interface node for this file path
      graphNode = currentData.nodes.find(n =>
        n.filePath === node.path && (n.type === 'file' || n.type === 'class' || n.type === 'interface') && !n.isVirtual
      );
      // Fallback: synthesize from file metadata (new pipeline only indexes methods, not class nodes)
      if (!graphNode) {
        const fileEntry = currentData.files.find(f => f.path === node.path);
        if (fileEntry) graphNode = fileNodeToGraphNode(fileEntry);
      }
    }

    if (!graphNode && node.type === 'directory') {
      graphNode = currentData.nodes.find(n =>
        n.type === 'directory' && !n.isVirtual && n.filePath === node.path
      );

      // Synthesize directly from the tree node. graphData.directories only contains
      // immediate-parent dirs of files, so intermediate dirs must be built from the tree node.
      if (!graphNode) {
        const dirEntry = currentData.directories.find(d => d.path === node.path);
        graphNode = dirEntry
          ? dirNodeToGraphNode(dirEntry)
          : { id: node.id, label: node.name, fileId: node.id, type: 'directory', filePath: node.path, line: 0, isVirtual: false };
      }
    }

    if (!graphNode) return;

    const existingVisibleNode = displayGraphData.nodes.find(n =>
      n.id === graphNode.id ||
      (graphNode.type === 'directory' && n.type === 'directory' && !n.isVirtual && n.filePath === graphNode.filePath)
    );

    if (existingVisibleNode) {
      // Already on canvas — just select
      selectNode(existingVisibleNode);
      return;
    }

    const displayNodeIds = new Set(displayGraphData.nodes.map(n => n.id));
    const connectedEdges = findConnectedEdges(currentData.edges, graphNode.id, displayNodeIds);

    setDisplayGraphData(prev => mergeGraphData(prev, {
      nodes: [graphNode],
      edges: connectedEdges,
    }));
    selectNode(graphNode);
  }, [getGraphData, loadGraph, displayGraphData.nodes, selectNode]);

  // Annotation update
  const handleAnnotationUpdate = useCallback(async () => {
    await handleExternalRefresh();
  }, [handleExternalRefresh]);

  // Virtual node created from InfoPanel form
  const handleNodeCreated = useCallback((node: GraphNode) => {
    addNodeToGraphData(node);
    setDisplayGraphData(prev => mergeGraphData(prev, { nodes: [node], edges: [] }));
    selectNode(node);
  }, [selectNode, addNodeToGraphData]);


  // Edge created from InfoPanel
  const handleEdgeCreated = useCallback((edge: GraphEdge) => {
    addEdgeToGraphData(edge);
    const sourceVisible = displayGraphData.nodes.some(n => n.id === edge.source);
    const targetVisible = displayGraphData.nodes.some(n => n.id === edge.target);
    if (sourceVisible && targetVisible) {
      setDisplayGraphData(prev => mergeGraphData(prev, { nodes: [], edges: [edge] }));
    }
    selectEdge(edge);
  }, [displayGraphData.nodes, selectEdge, addEdgeToGraphData]);

  // Virtual node deleted from InfoPanel
  const handleNodeDeleted = useCallback(async (nodeId: string) => {
    const success = await deleteVirtualNode(nodeId);
    if (success) {
      removeNodeFromGraphData(nodeId);
      setDisplayGraphData(prev => ({
        ...prev,
        nodes: prev.nodes.filter(n => n.id !== nodeId),
        edges: prev.edges.filter(e => e.source !== nodeId && e.target !== nodeId),
      }));
      clearSelection();
    } else {
      postMessage({ type: 'showError', text: 'Failed to delete node' });
    }
  }, [deleteVirtualNode, clearSelection, removeNodeFromGraphData]);

  // Edge deleted from InfoPanel
  const handleEdgeDeleted = useCallback(async (edgeId: string) => {
    const success = await deleteEdge(edgeId);
    if (success) {
      setDisplayGraphData(prev => ({
        ...prev,
        edges: prev.edges.filter(e => e.id !== edgeId),
      }));
      clearSelection();
    } else {
      postMessage({ type: 'showError', text: 'Failed to delete edge' });
    }
  }, [deleteEdge, clearSelection]);

  // Node deleted from canvas (Delete key)
  const handleNodeDelete = useCallback((node: GraphNode) => {
    if (node.isVirtual) {
      // Delegate to full delete (removes from backend too)
      handleNodeDeleted(node.id);
    } else {
      // Just remove from display canvas
      setDisplayGraphData(prev => ({
        ...prev,
        nodes: prev.nodes.filter(n => n.id !== node.id),
        edges: prev.edges.filter(e => e.source !== node.id && e.target !== node.id),
      }));
      clearSelection();
    }
  }, [handleNodeDeleted, clearSelection]);

  if (error) {
    return (
      <div className="app-error">
        <h2>Error Loading Graph</h2>
        <p>{error}</p>
        <button onClick={loadGraph}>Retry</button>
      </div>
    );
  }

  return (
    <div className="app">
      <header className="app-header">
        <h1>Graph-Viz</h1>
        <div className="header-actions">
          <button onClick={handleLoadAll} className="refresh-btn" title="Load All Nodes" disabled={loading}>
            {loading ? '⏳ Loading...' : '📥 Load All'}
          </button>
          <button onClick={handleClear} className="refresh-btn" title="Clear Graph">
            🗑️ Clear
          </button>
          <button onClick={() => refresh()} className="refresh-btn" title="Refresh Data" disabled={loading}>
            ↻ Refresh
          </button>
          <div className="snapshots-dropdown-container">
            <button onClick={() => setShowSnapshotsDropdown(!showSnapshotsDropdown)} className="refresh-btn" title="View Snapshots">
              📸 Snapshots
            </button>
            <SnapshotsDropdown isOpen={showSnapshotsDropdown} onClose={() => setShowSnapshotsDropdown(false)} onLoadSnapshot={handleLoadSnapshot} />
          </div>
          <div className="stats">
            <span>{displayGraphData.nodes.length} nodes</span>
            <span>{displayGraphData.edges.length} edges</span>
          </div>
        </div>
      </header>

      <div className="app-content">
        <ThreePanelLayout
          leftPanel={
            <FileTreePanel
              treeData={treeData}
              expandedNodes={expandedNodes}
              onToggle={toggleNode}
              onNodeClick={handleTreeNodeSelect}
              onAddToCanvas={handleAddToCanvas}
            />
          }
          centerPanel={
            <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
              <ShapePalette activeTool={placementMode} onToolSelect={setPlacementMode} />
              <GraphCanvas
                graphData={displayGraphData}
                placementMode={placementMode}
                selectedNodeId={selection.type === 'node' ? (selection.data as GraphNode).id : undefined}
                connectableNodeIds={connectableNodeIds}
                pendingNodePositionsRef={pendingNodePositionsRef}
                onNodeClick={handleNodeClick}
                onNodeDoubleClick={handleNodeDoubleClick}
                onNodeDelete={handleNodeDelete}
                onEdgeClick={handleEdgeClick}
                onBackgroundClick={handleBackgroundClick}
                onPlaceVirtualNode={handlePlaceVirtualNode}
                onNodeRightClickAddParent={handleNodeRightClickAddParent}
                onEdgeDrawn={handleEdgeDrawn}
              />
            </div>
          }
          rightPanel={
            <InfoPanel
              selectedNode={
                selection.type === 'node'
                  ? (graphData.nodes.find(n => n.id === (selection.data as GraphNode).id) ?? selection.data as GraphNode)
                  : null
              }
              selectedEdge={
                selection.type === 'edge'
                  ? (graphData.edges.find(e => e.id === (selection.data as GraphEdge).id) ?? selection.data as GraphEdge)
                  : null
              }
              existingNodes={graphData.nodes}
              activeNodes={displayGraphData.nodes}
              activeEdges={displayGraphData.edges}
              onAnnotationUpdate={handleAnnotationUpdate}
              onNodeCreated={handleNodeCreated}
              onEdgeCreated={handleEdgeCreated}
              onNodeDeleted={handleNodeDeleted}
              onEdgeDeleted={handleEdgeDeleted}
            />
          }
        />
      </div>
    </div>
  );
}

export default App;
