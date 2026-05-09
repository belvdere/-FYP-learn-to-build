/**
 * API client for communicating with the fypd backend.
 * Uses webview postMessage to talk to VS Code extension -> stdio to fypd.
 * No HTTP; all requests go through extension's handleApiRequest.
 */

import { GraphData, GraphNode, GraphEdge } from '../types/graph';
import {
  CreateVirtualNodeRequest,
  UpdateNodeAnnotationRequest,
  CreateEdgeRequest,
  UpdateEdgeAnnotationRequest,
  CreateVirtualDirectoryRequest,
  CreateSnapshotRequest,
  Snapshot,
  SnapshotDetail,
  SnapshotPromptResponse,
} from '../types/api';
import { 
  sendRequest, 
  isVSCodeWebview, 
  getVSCodeApi, 
  postMessage
} from './webviewTransport';

// Re-export utilities from webviewTransport for backward compatibility
export { isVSCodeWebview, getVSCodeApi };

/**
 * GraphApiClient - Uses webview message passing to communicate with backend
 */
export class GraphApiClient {
  constructor() {
    // No initialization needed - transport is handled by webviewTransport
  }

  // Health check
  async healthCheck(): Promise<boolean> {
    try {
      const result = await sendRequest<{ status: string }>('health.check');
      return result?.status === 'ok';
    } catch (error) {
      console.error('Health check failed:', error);
      return false;
    }
  }

  // Graph queries
  async getGraphData(): Promise<GraphData> {
    return sendRequest<GraphData>('graph.get');
  }

  async getNodeNeighborhood(nodeId: string): Promise<GraphData> {
    return sendRequest<GraphData>('graph.neighborhood', { nodeId });
  }

  async searchNodes(query: string): Promise<GraphNode[]> {
    return sendRequest<GraphNode[]>('graph.search', { q: query });
  }

  // File tree
  async getFileTree(): Promise<any> {
    return sendRequest('filetree.get');
  }

  // Node operations
  async getAllNodes(): Promise<GraphNode[]> {
    return sendRequest<GraphNode[]>('nodes.list');
  }

  async getNode(id: string): Promise<GraphNode> {
    return sendRequest<GraphNode>('nodes.get', { id });
  }

  async createVirtualNode(node: CreateVirtualNodeRequest): Promise<GraphNode> {
    return sendRequest<GraphNode>('nodes.create', {
      label: node.label,
      type: node.type,
      virtualClassId: node.virtualClassId,
      virtualDirectory: node.virtualDirectory
    });
  }

  async updateVirtualNode(id: string, node: Partial<CreateVirtualNodeRequest>): Promise<GraphNode> {
    return sendRequest<GraphNode>('nodes.update', {
      id,
      label: node.label,
      type: node.type,
      virtualClassId: node.virtualClassId,
      virtualDirectory: node.virtualDirectory
    });
  }

  async deleteVirtualNode(id: string): Promise<void> {
    await sendRequest('nodes.delete', { id });
  }

  // Edge operations
  async getAllEdges(): Promise<GraphEdge[]> {
    return sendRequest<GraphEdge[]>('edges.list');
  }

  async createEdge(edge: CreateEdgeRequest): Promise<GraphEdge> {
    return sendRequest<GraphEdge>('edges.create', {
      sourceNodeId: edge.sourceNodeId,
      targetNodeId: edge.targetNodeId,
      remarks: edge.remarks
    });
  }

  async deleteEdge(edgeId: string): Promise<void> {
    await sendRequest('edges.delete', { id: edgeId });
  }

  // Annotations
  async getNodeAnnotation(nodeId: string): Promise<any> {
    return sendRequest('annotations.node.get', { nodeId });
  }

  async updateNodeAnnotation(nodeId: string, annotation: UpdateNodeAnnotationRequest): Promise<void> {
    await sendRequest('annotations.node.update', {
      nodeId,
      description: annotation.description,
      aiRemarks: annotation.aiRemarks,
      codeSnippet: annotation.codeSnippet,
      customProperties: annotation.customProperties
    });
  }

  async getEdgeAnnotation(edgeId: string): Promise<any> {
    return sendRequest('annotations.edge.get', { edgeId });
  }

  async updateEdgeAnnotationDirect(edgeId: string, annotation: UpdateEdgeAnnotationRequest): Promise<void> {
    await sendRequest('annotations.edge.update', {
      edgeId,
      remarks: annotation.remarks
    });
  }

  // Virtual containers
  async createVirtualDirectory(dir: CreateVirtualDirectoryRequest): Promise<any> {
    return sendRequest('virtual.directories.create', {
      path: dir.path,
      name: dir.name,
      parentPath: dir.parentPath
    });
  }

  async deleteVirtualDirectory(id: string): Promise<void> {
    await sendRequest('virtual.directories.delete', { id });
  }

  // Snapshot operations (for agentic code generation)
  async createSnapshot(data: CreateSnapshotRequest): Promise<Snapshot> {
    return sendRequest<Snapshot>('snapshots.create', {
      name: data.name,
      virtualNodes: data.virtualNodes,
      contextNodes: data.contextNodes,
      edges: data.edges
    });
  }

  async listSnapshots(): Promise<Snapshot[]> {
    return sendRequest<Snapshot[]>('snapshots.list');
  }

  async getSnapshot(id: string): Promise<SnapshotDetail> {
    return sendRequest<SnapshotDetail>('snapshots.get', { id });
  }

  async deleteSnapshot(id: string): Promise<void> {
    await sendRequest('snapshots.delete', { id: String(id) });
  }

  async clearVirtualState(): Promise<void> {
    await sendRequest('graph.clearVirtual', {});
  }

  async loadSnapshot(id: string): Promise<{ restoredVirtualNodes: number; restoredEdges: number; missingContextNodes: number }> {
    return sendRequest('snapshots.load', { id });
  }

  // Snapshot prompt operations
  async getSnapshotPrompt(id: string): Promise<SnapshotPromptResponse> {
    return sendRequest<SnapshotPromptResponse>('snapshots.prompt.get', { id });
  }

  async updateSnapshotPrompt(id: string, prompt: string): Promise<void> {
    await sendRequest('snapshots.prompt.update', { id, prompt });
  }

  async resetSnapshotPrompt(id: string): Promise<SnapshotPromptResponse> {
    return sendRequest<SnapshotPromptResponse>('snapshots.prompt.reset', { id });
  }
}

// Create a default instance
export const graphApi = new GraphApiClient();

/**
 * Send a message to VS Code extension (for non-API messages)
 * Used for things like showInfo, showError, openMasking, etc.
 */
export function sendVSCodeMessage(message: unknown): void {
  postMessage(message);
}
