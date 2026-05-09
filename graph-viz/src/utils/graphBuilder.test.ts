import { describe, it, expect } from 'vitest';
import {
  buildCytoscapeElements,
  mergeGraphData,
  removeNodeFromGraph,
  removeEdgeFromGraph,
  findConnectedEdges,
  fileNodeToGraphNode,
} from './graphBuilder';
import { GraphData, GraphNode, GraphEdge, CytoscapeNode, CytoscapeEdge } from '../types/graph';

// Test fixtures
const createNode = (id: string, label: string = `Node ${id}`): GraphNode => ({
  id,
  label,
  fileId: `file-${id}`,
  type: 'method',
  filePath: `/path/to/${id}.java`,
  line: 10,
  isVirtual: false,
});

const createEdge = (id: string, source: string, target: string): GraphEdge => ({
  id,
  source,
  target,
  count: 1,
});

const emptyGraphData: GraphData = {
  nodes: [],
  edges: [],
  files: [],
  directories: [],
};

describe('mergeGraphData', () => {
  it('should merge nodes without duplicates', () => {
    const existing: GraphData = {
      ...emptyGraphData,
      nodes: [createNode('1'), createNode('2')],
    };

    const newData: Partial<GraphData> = {
      nodes: [createNode('2'), createNode('3')],  // '2' is duplicate
    };

    const result = mergeGraphData(existing, newData);

    expect(result.nodes).toHaveLength(3);
    expect(result.nodes.map(n => n.id)).toEqual(['1', '2', '3']);
  });

  it('should merge edges without duplicates', () => {
    const existing: GraphData = {
      ...emptyGraphData,
      edges: [createEdge('e1', '1', '2')],
    };

    const newData: Partial<GraphData> = {
      edges: [createEdge('e1', '1', '2'), createEdge('e2', '2', '3')],  // 'e1' is duplicate
    };

    const result = mergeGraphData(existing, newData);

    expect(result.edges).toHaveLength(2);
    expect(result.edges.map(e => e.id)).toEqual(['e1', 'e2']);
  });

  it('should handle empty new data', () => {
    const existing: GraphData = {
      ...emptyGraphData,
      nodes: [createNode('1')],
    };

    const result = mergeGraphData(existing, {});

    expect(result.nodes).toHaveLength(1);
    expect(result.edges).toHaveLength(0);
  });

  it('should handle empty existing data', () => {
    const newData: Partial<GraphData> = {
      nodes: [createNode('1'), createNode('2')],
    };

    const result = mergeGraphData(emptyGraphData, newData);

    expect(result.nodes).toHaveLength(2);
  });

  it('should merge files and directories', () => {
    const existing: GraphData = {
      nodes: [],
      edges: [],
      files: [{ id: 'f1', path: '/a.java', name: 'a.java', directory: 'd1', isVirtual: false }],
      directories: [{ id: 'd1', path: '/dir', name: 'dir', isVirtual: false }],
    };

    const newData: Partial<GraphData> = {
      files: [{ id: 'f2', path: '/b.java', name: 'b.java', directory: 'd1', isVirtual: false }],
      directories: [{ id: 'd2', path: '/dir2', name: 'dir2', isVirtual: false }],
    };

    const result = mergeGraphData(existing, newData);

    expect(result.files).toHaveLength(2);
    expect(result.directories).toHaveLength(2);
  });
});

describe('removeNodeFromGraph', () => {
  it('should remove the specified node', () => {
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [createNode('1'), createNode('2'), createNode('3')],
    };

    const result = removeNodeFromGraph(data, '2');

    expect(result.nodes).toHaveLength(2);
    expect(result.nodes.map(n => n.id)).toEqual(['1', '3']);
  });

  it('should remove edges connected to the removed node', () => {
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [createNode('1'), createNode('2'), createNode('3')],
      edges: [
        createEdge('e1', '1', '2'),  // connected to '2' - should be removed
        createEdge('e2', '2', '3'),  // connected to '2' - should be removed
        createEdge('e3', '1', '3'),  // not connected to '2' - should stay
      ],
    };

    const result = removeNodeFromGraph(data, '2');

    expect(result.edges).toHaveLength(1);
    expect(result.edges[0].id).toBe('e3');
  });

  it('should handle removing non-existent node', () => {
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [createNode('1')],
    };

    const result = removeNodeFromGraph(data, 'non-existent');

    expect(result.nodes).toHaveLength(1);
  });

  it('should preserve files and directories', () => {
    const data: GraphData = {
      nodes: [createNode('1')],
      edges: [],
      files: [{ id: 'f1', path: '/a.java', name: 'a.java', directory: 'd1', isVirtual: false }],
      directories: [{ id: 'd1', path: '/dir', name: 'dir', isVirtual: false }],
    };

    const result = removeNodeFromGraph(data, '1');

    expect(result.files).toHaveLength(1);
    expect(result.directories).toHaveLength(1);
  });
});

describe('removeEdgeFromGraph', () => {
  it('should remove the specified edge', () => {
    const data: GraphData = {
      ...emptyGraphData,
      edges: [createEdge('e1', '1', '2'), createEdge('e2', '2', '3')],
    };

    const result = removeEdgeFromGraph(data, 'e1');

    expect(result.edges).toHaveLength(1);
    expect(result.edges[0].id).toBe('e2');
  });

  it('should not affect nodes', () => {
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [createNode('1'), createNode('2')],
      edges: [createEdge('e1', '1', '2')],
    };

    const result = removeEdgeFromGraph(data, 'e1');

    expect(result.nodes).toHaveLength(2);
  });
});

describe('findConnectedEdges', () => {
  it('should find edges where node is source', () => {
    const edges = [
      createEdge('e1', 'A', 'B'),
      createEdge('e2', 'A', 'C'),
      createEdge('e3', 'B', 'C'),
    ];
    const existingNodeIds = new Set(['B', 'C']);

    const result = findConnectedEdges(edges, 'A', existingNodeIds);

    expect(result).toHaveLength(2);
    expect(result.map(e => e.id)).toContain('e1');
    expect(result.map(e => e.id)).toContain('e2');
  });

  it('should find edges where node is target', () => {
    const edges = [
      createEdge('e1', 'B', 'A'),
      createEdge('e2', 'C', 'A'),
      createEdge('e3', 'B', 'C'),
    ];
    const existingNodeIds = new Set(['B', 'C']);

    const result = findConnectedEdges(edges, 'A', existingNodeIds);

    expect(result).toHaveLength(2);
    expect(result.map(e => e.id)).toContain('e1');
    expect(result.map(e => e.id)).toContain('e2');
  });

  it('should only return edges where other endpoint exists', () => {
    const edges = [
      createEdge('e1', 'A', 'B'),
      createEdge('e2', 'A', 'X'),  // X not in existing nodes
    ];
    const existingNodeIds = new Set(['B']);

    const result = findConnectedEdges(edges, 'A', existingNodeIds);

    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('e1');
  });

  it('should return empty array when no connections exist', () => {
    const edges = [
      createEdge('e1', 'X', 'Y'),
    ];
    const existingNodeIds = new Set(['B']);

    const result = findConnectedEdges(edges, 'A', existingNodeIds);

    expect(result).toHaveLength(0);
  });
});

describe('buildCytoscapeElements', () => {
  it('should create elements for nodes', () => {
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [createNode('1'), createNode('2')],
    };

    const elements = buildCytoscapeElements(data);
    const nodeElements = elements.filter((e): e is CytoscapeNode => 'label' in e.data);

    expect(nodeElements).toHaveLength(2);
    expect(nodeElements[0].data.id).toBe('1');
    expect(nodeElements[0].data.label).toBe('Node 1');
  });

  it('should create elements for edges', () => {
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [createNode('1'), createNode('2')],
      edges: [createEdge('e1', '1', '2')],
    };

    const elements = buildCytoscapeElements(data);
    const edgeElements = elements.filter((e): e is CytoscapeEdge => 'source' in e.data);

    expect(edgeElements).toHaveLength(1);
    expect(edgeElements[0].data.source).toBe('1');
    expect(edgeElements[0].data.target).toBe('2');
  });

  it('should create separate edges for A->B and B->A (directed)', () => {
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [createNode('1'), createNode('2')],
      edges: [
        createEdge('e1', '1', '2'),
        createEdge('e2', '2', '1'),  // Opposite direction - now two separate edges
      ],
    };

    const elements = buildCytoscapeElements(data);

    // Two edges should be created (directed: A->B and B->A are distinct)
    const edgeElements = elements.filter((e): e is CytoscapeEdge => 'source' in e.data);
    expect(edgeElements).toHaveLength(2);
    expect(edgeElements[0].data.id).toBe('e1');
    expect(edgeElements[1].data.id).toBe('e2');
  });

  it('should not auto-materialize directories/files as canvas nodes', () => {
    const data: GraphData = {
      nodes: [],
      edges: [],
      files: [{ id: 'f1', path: '/a.java', name: 'a.java', directory: 'd1', isVirtual: false }],
      directories: [{ id: 'd1', path: '/dir', name: 'dir', isVirtual: false }],
    };

    const elements = buildCytoscapeElements(data);

    expect(elements).toHaveLength(0);
  });

  it('should set correct node types', () => {
    const virtualNode = { ...createNode('1'), isVirtual: true };
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [virtualNode, createNode('2')],
    };

    const elements = buildCytoscapeElements(data);
    const nodeElements = elements.filter((e): e is CytoscapeNode => 'label' in e.data);

    const virtualElement = nodeElements.find(e => e.data.id === '1');
    const methodElement = nodeElements.find(e => e.data.id === '2');

    expect(virtualElement?.data.type).toBe('virtual');
    expect(methodElement?.data.type).toBe('method');
  });

  it('should split method labels only for Java nodes', () => {
    const javaMethod: GraphNode = {
      ...createNode('java'),
      label: 'UserService.createUser',
      filePath: '/path/to/UserService.java',
    };
    const goMethod: GraphNode = {
      ...createNode('go'),
      label: 'Indexer.ScanFileSymbols',
      filePath: '/path/to/pipeline.go',
    };

    const data: GraphData = {
      ...emptyGraphData,
      nodes: [javaMethod, goMethod],
    };

    const elements = buildCytoscapeElements(data);
    const nodeElements = elements.filter((e): e is CytoscapeNode => 'label' in e.data);

    const javaEl = nodeElements.find(e => e.data.id === 'java');
    const goEl = nodeElements.find(e => e.data.id === 'go');

    expect(javaEl?.data.label).toBe('UserService\n.createUser');
    expect(goEl?.data.label).toBe('Indexer.ScanFileSymbols');
  });

  it('should nest a child under parent when parent is visible', () => {
    const fileNode: GraphNode = {
      id: 'file-1',
      label: 'UserService.java',
      fileId: 'file-1',
      type: 'class',
      filePath: '/src/UserService.java',
      line: 0,
      isVirtual: false,
    };
    const methodNode: GraphNode = {
      ...createNode('method-1', 'UserService.save'),
      parentId: 'file-1',
      filePath: '/src/UserService.java',
    };
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [fileNode, methodNode],
      files: [{ id: 'file-1', path: '/src/UserService.java', name: 'UserService.java', directory: 'dir-1', isVirtual: false }],
      directories: [{ id: 'dir-1', path: '/src', name: 'src', isVirtual: false }],
    };

    const elements = buildCytoscapeElements(data);
    const child = elements.find(e => e.data.id === 'method-1') as CytoscapeNode | undefined;
    expect(child?.data.parent).toBe('file-1');
  });

  it('should not set parent when the parent is not visible', () => {
    const methodNode: GraphNode = {
      ...createNode('method-1', 'UserService.save'),
      parentId: 'missing-file',
      filePath: '/src/UserService.java',
    };
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [methodNode],
      files: [{ id: 'missing-file', path: '/src/UserService.java', name: 'UserService.java', directory: 'dir-1', isVirtual: false }],
      directories: [{ id: 'dir-1', path: '/src', name: 'src', isVirtual: false }],
    };

    const elements = buildCytoscapeElements(data);
    const child = elements.find(e => e.data.id === 'method-1') as CytoscapeNode | undefined;
    expect(child?.data.parent).toBeUndefined();
  });

  it('should derive file parent from file metadata when directory is visible', () => {
    const dirNode: GraphNode = {
      id: 'dir-1',
      label: 'src',
      fileId: 'dir-1',
      type: 'directory',
      filePath: '/src',
      line: 0,
      isVirtual: false,
    };
    const fileNode: GraphNode = {
      id: 'file-1',
      label: 'UserService.java',
      fileId: 'file-1',
      type: 'class',
      filePath: '/src/UserService.java',
      line: 0,
      isVirtual: false,
    };
    const data: GraphData = {
      ...emptyGraphData,
      nodes: [dirNode, fileNode],
      files: [{ id: 'file-1', path: '/src/UserService.java', name: 'UserService.java', directory: 'dir-1', isVirtual: false }],
      directories: [{ id: 'dir-1', path: '/src', name: 'src', isVirtual: false }],
    };

    const elements = buildCytoscapeElements(data);
    const fileEl = elements.find(e => e.data.id === 'file-1') as CytoscapeNode | undefined;
    expect(fileEl?.data.parent).toBe('dir-1');
  });
});

describe('fileNodeToGraphNode', () => {
  it('should map file directory as parentId', () => {
    const result = fileNodeToGraphNode({
      id: 'file-1',
      path: '/src/UserService.java',
      name: 'UserService.java',
      directory: 'dir-1',
      isVirtual: false,
    });

    expect(result.parentId).toBe('dir-1');
  });
});
