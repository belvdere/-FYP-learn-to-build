// Custom hook for managing selection state

import { useState, useCallback } from 'react';
import { GraphNode, GraphEdge } from '../types/graph';

export type SelectionType = 'node' | 'edge' | null;

export interface Selection {
  type: SelectionType;
  data: GraphNode | GraphEdge | null;
}

export function useSelection() {
  const [selection, setSelection] = useState<Selection>({
    type: null,
    data: null,
  });

  const selectNode = useCallback((node: GraphNode) => {
    setSelection({
      type: 'node',
      data: node,
    });
  }, []);

  const selectEdge = useCallback((edge: GraphEdge) => {
    setSelection({
      type: 'edge',
      data: edge,
    });
  }, []);

  const clearSelection = useCallback(() => {
    setSelection({
      type: null,
      data: null,
    });
  }, []);

  return {
    selection,
    selectNode,
    selectEdge,
    clearSelection,
  };
}

