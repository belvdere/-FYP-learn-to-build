// Graph data types matching the backend API

export interface GraphNode {
  id: string;
  label: string;
  fileId: string;       // keep for backward compat (file tree panel)
  parentId?: string;    // compound parent for canvas hierarchy
  type: string; // "method", "class", "concept"
  filePath: string;
  line: number;
  isVirtual: boolean;
  materialized?: boolean;
  virtualClassId?: string;
  annotation?: NodeAnnotation;
  isStaleSnapshot?: boolean; // true = in snapshot but no longer in code index (read-only)
}

export interface GraphEdge {
  id: string;
  source: string;
  target: string;
  count: number;
  isManual?: boolean;
  annotation?: EdgeAnnotation;
  isStaleSnapshot?: boolean; // true = in snapshot but no longer in code index (read-only)
}

export interface NodeAnnotation {
  description?: string;
  aiRemarks?: string;
  codeSnippet?: string;
  customProperties?: string; // JSON string
}

export interface EdgeAnnotation {
  remarks?: string;
}

export interface FileNode {
  id: string;
  path: string;
  name: string;
  directory: string;
  isVirtual: boolean;
}

export interface DirectoryNode {
  id: string;
  path: string;
  name: string;
  parentPath?: string;
  isVirtual: boolean;
}

export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
  files: FileNode[];
  directories: DirectoryNode[];
}

// Cytoscape element types
export type CytoscapeNodeType =
  | 'directory'
  | 'file'
  | 'method'
  | 'field'
  | 'virtual'
  | 'class'
  | 'virtual-class'
  | 'virtual-directory';

export interface CytoscapeNode {
  data: {
    id: string;
    label: string;
    type: CytoscapeNodeType;
    parent?: string;
    isVirtual?: boolean;
    isStaleSnapshot?: boolean;
    nodeData?: GraphNode;
  };
}

export interface CytoscapeEdge {
  data: {
    id: string;
    source: string;
    target: string;
    count?: number;
    isVirtual?: boolean;
    isStaleSnapshot?: boolean;
    edgeData?: GraphEdge;
  };
}

export type CytoscapeElement = CytoscapeNode | CytoscapeEdge;
