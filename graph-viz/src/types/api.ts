// API request/response types

export interface CreateVirtualNodeRequest {
  label: string;
  type: string;
  virtualClassId?: string;
  virtualDirectory?: string;
}

export interface UpdateNodeAnnotationRequest {
  description?: string;
  aiRemarks?: string;
  codeSnippet?: string;
  customProperties?: string;
}

export interface CreateEdgeRequest {
  sourceNodeId: string;
  targetNodeId: string;
  remarks?: string;
}

export interface UpdateEdgeAnnotationRequest {
  remarks?: string;
}

export interface CreateVirtualDirectoryRequest {
  path: string;
  name: string;
  parentPath?: string;
}

export interface ApiError {
  error: string;
  message?: string;
}

// Snapshot types (for agentic code generation)

export interface SnapshotNode {
  id: string;
  label: string;
  type: string;
  filePath?: string;
  line?: number;
  isVirtual: boolean;
  parentId?: string;
  description?: string;
  aiRemarks?: string;
}

export interface SnapshotEdge {
  id: string;
  sourceId: string;
  targetId: string;
  remarks?: string;
}

export interface CreateSnapshotRequest {
  name?: string;
  virtualNodes: SnapshotNode[];
  contextNodes: SnapshotNode[];
  edges: SnapshotEdge[];
}

export interface Snapshot {
  id: string;
  name?: string;
  virtualNodeCount: number;
  contextNodeCount: number;
  edgeCount: number;
  createdAt: number;
}

export interface SnapshotDetail {
  id: string;
  name?: string;
  virtualNodes: SnapshotNode[];
  contextNodes: SnapshotNode[];
  edges: SnapshotEdge[];
  createdAt: number;
}

// Snapshot prompt types

export interface SnapshotPromptResponse {
  prompt: string;
  isCustom: boolean;
}

export interface UpdateSnapshotPromptRequest {
  prompt: string;
}

