import { describe, it, expect } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useSelection } from './useSelection';
import { GraphNode, GraphEdge } from '../types/graph';

const mockNode: GraphNode = {
  id: 'node-1',
  label: 'Test Node',
  fileId: 'file-1',
  type: 'method',
  filePath: '/path/to/file.java',
  line: 42,
  isVirtual: false,
};

const mockEdge: GraphEdge = {
  id: 'edge-1',
  source: 'node-1',
  target: 'node-2',
  count: 5,
};

describe('useSelection', () => {
  it('should start with no selection', () => {
    const { result } = renderHook(() => useSelection());

    expect(result.current.selection.type).toBeNull();
    expect(result.current.selection.data).toBeNull();
  });

  it('should select a node', () => {
    const { result } = renderHook(() => useSelection());

    act(() => {
      result.current.selectNode(mockNode);
    });

    expect(result.current.selection.type).toBe('node');
    expect(result.current.selection.data).toEqual(mockNode);
  });

  it('should select an edge', () => {
    const { result } = renderHook(() => useSelection());

    act(() => {
      result.current.selectEdge(mockEdge);
    });

    expect(result.current.selection.type).toBe('edge');
    expect(result.current.selection.data).toEqual(mockEdge);
  });

  it('should clear selection', () => {
    const { result } = renderHook(() => useSelection());

    // First select something
    act(() => {
      result.current.selectNode(mockNode);
    });

    expect(result.current.selection.type).toBe('node');

    // Then clear
    act(() => {
      result.current.clearSelection();
    });

    expect(result.current.selection.type).toBeNull();
    expect(result.current.selection.data).toBeNull();
  });

  it('should replace node selection with edge selection', () => {
    const { result } = renderHook(() => useSelection());

    act(() => {
      result.current.selectNode(mockNode);
    });

    expect(result.current.selection.type).toBe('node');

    act(() => {
      result.current.selectEdge(mockEdge);
    });

    expect(result.current.selection.type).toBe('edge');
    expect(result.current.selection.data).toEqual(mockEdge);
  });

  it('should replace edge selection with node selection', () => {
    const { result } = renderHook(() => useSelection());

    act(() => {
      result.current.selectEdge(mockEdge);
    });

    expect(result.current.selection.type).toBe('edge');

    act(() => {
      result.current.selectNode(mockNode);
    });

    expect(result.current.selection.type).toBe('node');
    expect(result.current.selection.data).toEqual(mockNode);
  });

  it('should handle selecting different nodes', () => {
    const { result } = renderHook(() => useSelection());

    const node1 = { ...mockNode, id: 'node-1', label: 'Node 1' };
    const node2 = { ...mockNode, id: 'node-2', label: 'Node 2' };

    act(() => {
      result.current.selectNode(node1);
    });

    expect((result.current.selection.data as GraphNode).id).toBe('node-1');

    act(() => {
      result.current.selectNode(node2);
    });

    expect((result.current.selection.data as GraphNode).id).toBe('node-2');
    expect((result.current.selection.data as GraphNode).label).toBe('Node 2');
  });
});

