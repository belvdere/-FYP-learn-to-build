/**
 * Transform API data to Cytoscape format for rendering.
 * Flat canvas — no compound/parent hierarchy. All nodes are standalone.
 */
import { GraphData, GraphNode, GraphEdge, FileNode, DirectoryNode, CytoscapeElement, CytoscapeNodeType } from '../types/graph';

/**
 * Format a node label to display on two lines:
 * Line 1: words before the last dot
 * Line 2: dot + words after the last dot
 * e.g., "FileController.shouldTranslate" -> "FileController\n.shouldTranslate"
 */
function formatNodeLabel(label: string): string {
  const lastDotIndex = label.lastIndexOf('.');
  if (lastDotIndex === -1 || lastDotIndex === 0) {
    return label;
  }
  const beforeDot = label.substring(0, lastDotIndex);
  const afterDot = label.substring(lastDotIndex);
  return `${beforeDot}\n${afterDot}`;
}

function shouldUseJavaDotSplit(node: GraphNode, nodeType: CytoscapeNodeType): boolean {
  const isCallableLike = nodeType === 'method' || nodeType === 'virtual' || nodeType === 'field';
  if (!isCallableLike) return false;
  return typeof node.filePath === 'string' && node.filePath.toLowerCase().endsWith('.java');
}

/**
 * Compute the Cytoscape node type from a GraphNode.
 * File and class are merged into 'file' (one .java file = one class).
 */
function computeNodeType(node: GraphNode): CytoscapeNodeType {
  if (node.isVirtual) {
    if (node.type === 'class') return 'virtual-class';
    if (node.type === 'directory') return 'virtual-directory';
    return 'virtual';
  }
  if (node.type === 'directory') return 'directory';
  if (node.type === 'class' || node.type === 'interface') return 'file';
  if (node.type === 'field') return 'field';
  return 'method';
}

const CONTAINER_NODE_TYPES = new Set<CytoscapeNodeType>([
  'directory',
  'file',
  'virtual-directory',
  'virtual-class',
]);

function resolveEffectiveParentId(
  node: GraphNode,
  nodeType: CytoscapeNodeType,
  visibleNodesByID: Map<string, GraphNode>,
  nodeTypeByID: Map<string, CytoscapeNodeType>,
  fileByPath: Map<string, FileNode>,
): string | undefined {
  let candidateParentID = node.parentId;

  // Real node fallback when parentId is absent:
  // - method/field -> file
  // - file/class/interface -> directory
  if (!candidateParentID && !node.isVirtual && node.filePath) {
    const file = fileByPath.get(node.filePath);
    if (nodeType === 'method' || nodeType === 'field') {
      candidateParentID = file?.id;
    } else if (nodeType === 'file') {
      candidateParentID = file?.directory;
    }
  }

  if (!candidateParentID || candidateParentID === node.id) return undefined;

  const visibleParent = visibleNodesByID.get(candidateParentID);
  if (!visibleParent) return undefined;

  const parentType = nodeTypeByID.get(candidateParentID) ?? computeNodeType(visibleParent);
  if (!CONTAINER_NODE_TYPES.has(parentType)) return undefined;

  return candidateParentID;
}

/**
 * Build Cytoscape elements from graph data.
 * Uses conditional parent assignment:
 * if a valid parent node is currently visible, the child is rendered inside it.
 */
export function buildCytoscapeElements(data: GraphData): CytoscapeElement[] {
  const elements: CytoscapeElement[] = [];
  const addedIds = new Set<string>();
  const addedEdges = new Set<string>();
  const visibleNodesByID = new Map<string, GraphNode>(data.nodes.map(node => [node.id, node]));
  const fileByPath = new Map<string, FileNode>(data.files.map(file => [file.path, file]));
  const nodeTypeByID = new Map<string, CytoscapeNodeType>();

  data.nodes.forEach(node => {
    nodeTypeByID.set(node.id, computeNodeType(node));
  });

  data.nodes.forEach(node => {
    if (addedIds.has(node.id)) return;
    const nodeType = nodeTypeByID.get(node.id) || computeNodeType(node);
    const effectiveParentID = resolveEffectiveParentId(
      node,
      nodeType,
      visibleNodesByID,
      nodeTypeByID,
      fileByPath
    );
    const displayLabel = shouldUseJavaDotSplit(node, nodeType)
      ? formatNodeLabel(node.label)
      : node.label;
    elements.push({
      data: {
        id: node.id,
        label: displayLabel,
        type: nodeType,
        ...(effectiveParentID ? { parent: effectiveParentID } : {}),
        isVirtual: node.isVirtual,
        isStaleSnapshot: node.isStaleSnapshot || false,
        nodeData: node,
      },
    });
    addedIds.add(node.id);
  });

  // Build set of virtual node IDs so we can tag user-created edges
  const virtualNodeIds = new Set<string>();
  data.nodes.forEach(node => {
    if (node.isVirtual) virtualNodeIds.add(node.id);
  });

  data.edges.forEach(edge => {
    if (addedEdges.has(edge.id)) return;
    // Skip edges whose endpoints haven't been added — Cytoscape throws on dangling edges
    if (!addedIds.has(edge.source) || !addedIds.has(edge.target)) return;
    const isUserCreated = edge.isManual || virtualNodeIds.has(edge.source) || virtualNodeIds.has(edge.target);
    elements.push({
      data: {
        id: edge.id,
        source: edge.source,
        target: edge.target,
        count: edge.count,
        isVirtual: isUserCreated,
        isStaleSnapshot: edge.isStaleSnapshot || false,
        edgeData: edge,
      },
    });
    addedEdges.add(edge.id);
  });

  return elements;
}

/**
 * Build a GraphNode from a FileNode (for right-click add-parent).
 */
export function fileNodeToGraphNode(file: FileNode): GraphNode {
  return {
    id: file.id,
    label: file.name,
    fileId: file.id,
    parentId: file.directory,
    type: 'class',
    filePath: file.path,
    line: 0,
    isVirtual: false,
  };
}

/**
 * Build a GraphNode from a DirectoryNode (for right-click add-parent).
 */
export function dirNodeToGraphNode(dir: DirectoryNode): GraphNode {
  return {
    id: dir.id,
    label: dir.name,
    fileId: dir.id,
    type: 'directory',
    filePath: dir.path,
    line: 0,
    isVirtual: false,
  };
}

/**
 * Merge new graph data into existing data with deduplication.
 */
export function mergeGraphData(existing: GraphData, newData: Partial<GraphData>): GraphData {
  const existingNodeIds = new Set(existing.nodes.map(n => n.id));
  const existingEdgeIds = new Set(existing.edges.map(e => e.id));
  const existingFileIds = new Set(existing.files.map(f => f.id));
  const existingDirIds = new Set(existing.directories.map(d => d.id));

  return {
    nodes: [
      ...existing.nodes,
      ...(newData.nodes || []).filter(n => !existingNodeIds.has(n.id)),
    ],
    edges: [
      ...existing.edges,
      ...(newData.edges || []).filter(e => !existingEdgeIds.has(e.id)),
    ],
    files: [
      ...existing.files,
      ...(newData.files || []).filter(f => !existingFileIds.has(f.id)),
    ],
    directories: [
      ...existing.directories,
      ...(newData.directories || []).filter(d => !existingDirIds.has(d.id)),
    ],
  };
}

/**
 * Remove a node and all its connected edges from the graph.
 */
export function removeNodeFromGraph(data: GraphData, nodeId: string): GraphData {
  return {
    nodes: data.nodes.filter(n => n.id !== nodeId),
    edges: data.edges.filter(e => e.source !== nodeId && e.target !== nodeId),
    files: data.files,
    directories: data.directories,
  };
}

/**
 * Remove an edge from the graph.
 */
export function removeEdgeFromGraph(data: GraphData, edgeId: string): GraphData {
  return {
    ...data,
    edges: data.edges.filter(e => e.id !== edgeId),
  };
}

/**
 * Find edges connected to a specific node where the other endpoint exists in the given node set.
 */
export function findConnectedEdges(
  allEdges: GraphEdge[],
  nodeId: string,
  existingNodeIds: Set<string>
): GraphEdge[] {
  return allEdges.filter(edge =>
    (edge.source === nodeId && existingNodeIds.has(edge.target)) ||
    (edge.target === nodeId && existingNodeIds.has(edge.source))
  );
}
