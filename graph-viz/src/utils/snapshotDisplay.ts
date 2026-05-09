import { GraphData, GraphEdge, GraphNode } from '../types/graph';
import { SnapshotDetail, SnapshotNode } from '../types/api';
import { dirNodeToGraphNode, fileNodeToGraphNode } from './graphBuilder';

function isDirectoryContextNode(node: SnapshotNode): boolean {
  return !node.isVirtual && node.type === 'directory';
}

function isFileContextNode(node: SnapshotNode): boolean {
  if (node.isVirtual) return false;
  if (node.type === 'file') return true;
  // Backward-compat: older snapshots stored file placeholders as class/interface with line=0.
  return (node.type === 'class' || node.type === 'interface') && !!node.filePath && (node.line ?? 0) === 0;
}

function withSnapshotAnnotation(node: GraphNode, snapshotNode: SnapshotNode): GraphNode {
  if (node.annotation || (!snapshotNode.description && !snapshotNode.aiRemarks)) {
    return node;
  }
  return {
    ...node,
    annotation: {
      description: snapshotNode.description,
      aiRemarks: snapshotNode.aiRemarks,
    },
  };
}

function createStalePlaceholder(snapshotNode: SnapshotNode): GraphNode {
  const mappedType = snapshotNode.type === 'file' ? 'class' : (snapshotNode.type || 'method');
  return {
    id: snapshotNode.id,
    label: snapshotNode.label,
    fileId: snapshotNode.filePath || snapshotNode.id,
    parentId: snapshotNode.parentId,
    type: mappedType,
    filePath: snapshotNode.filePath || '',
    line: snapshotNode.line || 0,
    isVirtual: snapshotNode.isVirtual,
    isStaleSnapshot: true,
    annotation: (snapshotNode.description || snapshotNode.aiRemarks)
      ? { description: snapshotNode.description, aiRemarks: snapshotNode.aiRemarks }
      : undefined,
  };
}

/**
 * Builds display data from a freshly-loaded graph + snapshot detail.
 * Nodes/edges in the snapshot but missing from the fresh graph are marked isStaleSnapshot=true.
 */
export function buildSnapshotDisplayData(freshData: GraphData, snapshot: SnapshotDetail): GraphData {
  const freshNodeMap = new Map<string, GraphNode>(freshData.nodes.map(n => [n.id, n]));
  const freshFileByID = new Map(freshData.files.map(f => [f.id, f]));
  const freshFileByPath = new Map(freshData.files.map(f => [f.path, f]));
  const freshDirByID = new Map(freshData.directories.map(d => [d.id, d]));
  const freshDirByPath = new Map(freshData.directories.map(d => [d.path, d]));
  const freshEdgeMap = new Map<string, GraphEdge>(freshData.edges.map(e => [`${e.source}::${e.target}`, e]));

  const allSnapshotNodes = [...snapshot.virtualNodes, ...snapshot.contextNodes];

  const nodes: GraphNode[] = [];
  const nodeIDs = new Set<string>();
  const resolvedIDBySnapshotID = new Map<string, string>();

  for (const sn of allSnapshotNodes) {
    let resolvedNode: GraphNode | undefined = freshNodeMap.get(sn.id);

    if (!resolvedNode && isDirectoryContextNode(sn)) {
      const dir = freshDirByID.get(sn.id) || (sn.filePath ? freshDirByPath.get(sn.filePath) : undefined);
      if (dir) {
        resolvedNode = withSnapshotAnnotation(dirNodeToGraphNode(dir), sn);
      }
    }

    if (!resolvedNode && isFileContextNode(sn)) {
      const file = freshFileByID.get(sn.id) || (sn.filePath ? freshFileByPath.get(sn.filePath) : undefined);
      if (file) {
        resolvedNode = withSnapshotAnnotation(fileNodeToGraphNode(file), sn);
      }
    }

    if (!resolvedNode) {
      resolvedNode = createStalePlaceholder(sn);
    }

    resolvedIDBySnapshotID.set(sn.id, resolvedNode.id);

    if (!nodeIDs.has(resolvedNode.id)) {
      nodes.push(resolvedNode);
      nodeIDs.add(resolvedNode.id);
    }
  }

  const edges: GraphEdge[] = [];
  const edgeIDs = new Set<string>();
  for (const se of snapshot.edges) {
    const sourceID = resolvedIDBySnapshotID.get(se.sourceId) || se.sourceId;
    const targetID = resolvedIDBySnapshotID.get(se.targetId) || se.targetId;

    if (!nodeIDs.has(sourceID) || !nodeIDs.has(targetID)) continue;

    const edgeKey = `${sourceID}::${targetID}`;
    if (edgeIDs.has(edgeKey)) continue;

    const freshEdge = freshEdgeMap.get(edgeKey);
    if (freshEdge) {
      edges.push(
        (!freshEdge.annotation && se.remarks)
          ? { ...freshEdge, annotation: { remarks: se.remarks } }
          : freshEdge
      );
    } else {
      edges.push({
        id: edgeKey,
        source: sourceID,
        target: targetID,
        count: 1,
        annotation: se.remarks ? { remarks: se.remarks } : undefined,
        isStaleSnapshot: true,
      });
    }

    edgeIDs.add(edgeKey);
  }

  return {
    nodes,
    edges,
    files: freshData.files,
    directories: freshData.directories,
  };
}
