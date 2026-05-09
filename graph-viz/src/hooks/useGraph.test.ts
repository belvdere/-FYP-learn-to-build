import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useGraph } from './useGraph';
import { graphApi } from '../services/graphApi';
import { GraphData } from '../types/graph';

// Mock the graphApi
vi.mock('../services/graphApi', () => ({
  graphApi: {
    getGraphData: vi.fn(),
    getNodeNeighborhood: vi.fn(),
    searchNodes: vi.fn(),
  },
}));

const mockGraphData: GraphData = {
  nodes: [
    {
      id: '1',
      label: 'Node 1',
      fileId: 'f1',
      type: 'method',
      filePath: '/path/1.java',
      line: 10,
      isVirtual: false,
    },
    {
      id: '2',
      label: 'Node 2',
      fileId: 'f1',
      type: 'method',
      filePath: '/path/2.java',
      line: 20,
      isVirtual: false,
    },
  ],
  edges: [
    { id: 'e1', source: '1', target: '2', count: 1 },
  ],
  files: [],
  directories: [],
};

describe('useGraph', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('loadGraph', () => {
    it('should load graph data and return it', async () => {
      vi.mocked(graphApi.getGraphData).mockResolvedValue(mockGraphData);

      const { result } = renderHook(() => useGraph());

      expect(result.current.graphData.nodes).toHaveLength(0);

      let returnedData: GraphData | undefined;
      await act(async () => {
        returnedData = await result.current.loadGraph();
      });

      expect(graphApi.getGraphData).toHaveBeenCalledOnce();
      expect(result.current.graphData.nodes).toHaveLength(2);
      expect(returnedData).toEqual(mockGraphData);
    });

    it('should set loading state during fetch', async () => {
      vi.mocked(graphApi.getGraphData).mockImplementation(
        () => new Promise(resolve => setTimeout(() => resolve(mockGraphData), 100))
      );

      const { result } = renderHook(() => useGraph());

      expect(result.current.loading).toBe(false);

      let loadPromise: Promise<GraphData>;
      act(() => {
        loadPromise = result.current.loadGraph();
      });

      expect(result.current.loading).toBe(true);

      await act(async () => {
        await loadPromise;
      });

      expect(result.current.loading).toBe(false);
    });

    it('should handle errors', async () => {
      vi.mocked(graphApi.getGraphData).mockRejectedValue(new Error('Network error'));

      const { result } = renderHook(() => useGraph());

      await act(async () => {
        await result.current.loadGraph();
      });

      expect(result.current.error).toBe('Network error');
      expect(result.current.graphData.nodes).toHaveLength(0);
    });
  });

  describe('expandNode', () => {
    it('should merge neighbor data and return updated graph', async () => {
      const neighborData: GraphData = {
        nodes: [
          {
            id: '3',
            label: 'Node 3',
            fileId: 'f2',
            type: 'method',
            filePath: '/path/3.java',
            line: 30,
            isVirtual: false,
          },
        ],
        edges: [
          { id: 'e2', source: '1', target: '3', count: 1 },
        ],
        files: [],
        directories: [],
      };

      vi.mocked(graphApi.getGraphData).mockResolvedValue(mockGraphData);
      vi.mocked(graphApi.getNodeNeighborhood).mockResolvedValue(neighborData);

      const { result } = renderHook(() => useGraph());

      // First load the graph
      await act(async () => {
        await result.current.loadGraph();
      });

      expect(result.current.graphData.nodes).toHaveLength(2);

      // Then expand a node
      let expandedData: GraphData | undefined;
      await act(async () => {
        expandedData = await result.current.expandNode('1');
      });

      expect(result.current.graphData.nodes).toHaveLength(3);
      expect(result.current.graphData.edges).toHaveLength(2);
      expect(expandedData?.nodes).toHaveLength(3);
    });

    it('should not create duplicates when expanding', async () => {
      // Neighbor data includes a node that already exists
      const neighborData: GraphData = {
        nodes: [
          {
            id: '2',  // Duplicate!
            label: 'Node 2',
            fileId: 'f1',
            type: 'method',
            filePath: '/path/2.java',
            line: 20,
            isVirtual: false,
          },
        ],
        edges: [],
        files: [],
        directories: [],
      };

      vi.mocked(graphApi.getGraphData).mockResolvedValue(mockGraphData);
      vi.mocked(graphApi.getNodeNeighborhood).mockResolvedValue(neighborData);

      const { result } = renderHook(() => useGraph());

      await act(async () => {
        await result.current.loadGraph();
      });

      await act(async () => {
        await result.current.expandNode('1');
      });

      // Should still be 2 nodes, not 3
      expect(result.current.graphData.nodes).toHaveLength(2);
    });
  });

  describe('removeNode', () => {
    it('should remove node and return updated data', async () => {
      vi.mocked(graphApi.getGraphData).mockResolvedValue(mockGraphData);

      const { result } = renderHook(() => useGraph());

      await act(async () => {
        await result.current.loadGraph();
      });

      let updatedData: GraphData | undefined;
      act(() => {
        updatedData = result.current.removeNode('1');
      });

      expect(result.current.graphData.nodes).toHaveLength(1);
      expect(result.current.graphData.nodes[0].id).toBe('2');
      expect(updatedData?.nodes).toHaveLength(1);
    });

    it('should remove connected edges when removing node', async () => {
      vi.mocked(graphApi.getGraphData).mockResolvedValue(mockGraphData);

      const { result } = renderHook(() => useGraph());

      await act(async () => {
        await result.current.loadGraph();
      });

      expect(result.current.graphData.edges).toHaveLength(1);

      act(() => {
        result.current.removeNode('1');
      });

      expect(result.current.graphData.edges).toHaveLength(0);
    });
  });

  describe('getGraphData', () => {
    it('should return current graph data', async () => {
      vi.mocked(graphApi.getGraphData).mockResolvedValue(mockGraphData);

      const { result } = renderHook(() => useGraph());

      await act(async () => {
        await result.current.loadGraph();
      });

      const data = result.current.getGraphData();

      expect(data).toEqual(result.current.graphData);
      expect(data.nodes).toHaveLength(2);
    });
  });

  describe('addVirtualNode', () => {
    it('should add a virtual node', async () => {
      const { result } = renderHook(() => useGraph());

      const virtualNode = {
        id: 'v1',
        label: 'Virtual Node',
        fileId: 'vf1',
        type: 'concept',
        filePath: '',
        line: 0,
        isVirtual: true,
      };

      act(() => {
        result.current.addVirtualNode(virtualNode);
      });

      expect(result.current.graphData.nodes).toHaveLength(1);
      expect(result.current.graphData.nodes[0].isVirtual).toBe(true);
    });
  });
});

