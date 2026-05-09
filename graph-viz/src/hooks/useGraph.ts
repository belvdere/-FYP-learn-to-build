/**
 * useGraph - Manages graph state and API operations.
 * graphData = full graph from API (for file tree, search). displayGraphData is managed by App.
 * Uses graphDataRef to avoid stale closures in async callbacks.
 */
import { useState, useCallback, useRef } from 'react';
import { GraphData, GraphNode, GraphEdge } from '../types/graph';
import { graphApi } from '../services/graphApi';
import { mergeGraphData, removeNodeFromGraph, removeEdgeFromGraph } from '../utils/graphBuilder';
import { CreateVirtualNodeRequest, CreateEdgeRequest } from '../types/api';

const EMPTY_GRAPH_DATA: GraphData = {
  nodes: [],
  edges: [],
  files: [],
  directories: [],
};

export function useGraph() {
  const [graphData, setGraphData] = useState<GraphData>(EMPTY_GRAPH_DATA);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Use ref to always have access to latest graphData in async callbacks
  const graphDataRef = useRef<GraphData>(graphData);
  graphDataRef.current = graphData;

  // Load initial graph data - returns the fetched data for immediate use
  const loadGraph = useCallback(async (): Promise<GraphData> => {
    setLoading(true);
    setError(null);
    try {
      const data = await graphApi.getGraphData();
      setGraphData(data);
      return data;
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to load graph data';
      setError(errorMessage);
      console.error('Failed to load graph:', err);
      return EMPTY_GRAPH_DATA;
    } finally {
      setLoading(false);
    }
  }, []);

  // Expand node neighbors - returns the merged graph data
  const expandNode = useCallback(async (nodeId: string): Promise<GraphData> => {
    try {
      const neighborData = await graphApi.getNodeNeighborhood(nodeId);
      const merged = mergeGraphData(graphDataRef.current, neighborData);
      setGraphData(merged);
      return merged;
    } catch (err) {
      console.error('Failed to expand node:', err);
      return graphDataRef.current;
    }
  }, []);

  // Search and add nodes - returns found nodes
  const searchAndAddNodes = useCallback(async (query: string): Promise<GraphNode[]> => {
    try {
      const nodes = await graphApi.searchNodes(query);
      // Add found nodes to graph
      const merged = mergeGraphData(graphDataRef.current, { nodes });
      setGraphData(merged);
      return nodes;
    } catch (err) {
      console.error('Failed to search nodes:', err);
      return [];
    }
  }, []);

  // Remove node from graph - returns updated graph data
  const removeNode = useCallback((nodeId: string): GraphData => {
    const updated = removeNodeFromGraph(graphDataRef.current, nodeId);
    setGraphData(updated);
    return updated;
  }, []);

  // Remove edge from graph - returns updated graph data
  const removeEdge = useCallback((edgeId: string): GraphData => {
    const updated = removeEdgeFromGraph(graphDataRef.current, edgeId);
    setGraphData(updated);
    return updated;
  }, []);

  // Add virtual node to local state only - returns updated graph data
  const addVirtualNode = useCallback((node: GraphNode): GraphData => {
    const updated = {
      ...graphDataRef.current,
      nodes: [...graphDataRef.current.nodes, node],
    };
    setGraphData(updated);
    return updated;
  }, []);

  // Create virtual node via API and add to state - returns the created node
  const createVirtualNode = useCallback(async (request: CreateVirtualNodeRequest): Promise<GraphNode | null> => {
    try {
      const createdNode = await graphApi.createVirtualNode(request);
      // Add to local state
      const updated = {
        ...graphDataRef.current,
        nodes: [...graphDataRef.current.nodes, createdNode],
      };
      setGraphData(updated);
      return createdNode;
    } catch (err) {
      console.error('Failed to create virtual node:', err);
      return null;
    }
  }, []);

  // Create edge via API and add to state - returns the created edge
  const createEdge = useCallback(async (request: CreateEdgeRequest): Promise<GraphEdge | null> => {
    try {
      const createdEdge = await graphApi.createEdge(request);
      // Add to local state
      const updated = {
        ...graphDataRef.current,
        edges: [...graphDataRef.current.edges, createdEdge],
      };
      setGraphData(updated);
      return createdEdge;
    } catch (err) {
      console.error('Failed to create edge:', err);
      return null;
    }
  }, []);

  // Delete virtual node via API and remove from state
  const deleteVirtualNode = useCallback(async (nodeId: string): Promise<boolean> => {
    try {
      await graphApi.deleteVirtualNode(nodeId);
      // Remove from local state
      const updated = removeNodeFromGraph(graphDataRef.current, nodeId);
      setGraphData(updated);
      return true;
    } catch (err) {
      console.error('Failed to delete virtual node:', err);
      return false;
    }
  }, []);

  // Delete edge via API and remove from state
  const deleteEdge = useCallback(async (edgeId: string): Promise<boolean> => {
    try {
      await graphApi.deleteEdge(edgeId);
      // Remove from local state
      const updated = removeEdgeFromGraph(graphDataRef.current, edgeId);
      setGraphData(updated);
      return true;
    } catch (err) {
      console.error('Failed to delete edge:', err);
      return false;
    }
  }, []);

  // Refresh graph data - returns the refreshed data
  const refresh = useCallback(async (): Promise<GraphData> => {
    return loadGraph();
  }, [loadGraph]);

  // Get current graph data (useful for accessing latest state without closures)
  const getGraphData = useCallback((): GraphData => {
    return graphDataRef.current;
  }, []);

  // Add a node to the main graphData (no-op if the node already exists)
  const addNodeToGraphData = useCallback((node: GraphNode): void => {
    if (graphDataRef.current.nodes.some(n => n.id === node.id)) return;
    setGraphData({
      ...graphDataRef.current,
      nodes: [...graphDataRef.current.nodes, node],
    });
  }, []);

  // Add an edge to the main graphData (no-op if the edge already exists)
  const addEdgeToGraphData = useCallback((edge: GraphEdge): void => {
    if (graphDataRef.current.edges.some(e => e.id === edge.id)) return;
    setGraphData({
      ...graphDataRef.current,
      edges: [...graphDataRef.current.edges, edge],
    });
  }, []);

  // Remove a node from the main graphData (for file tree updates)
  const removeNodeFromGraphData = useCallback((nodeId: string): void => {
    const updated = removeNodeFromGraph(graphDataRef.current, nodeId);
    setGraphData(updated);
  }, []);

  return {
    graphData,
    loading,
    error,
    loadGraph,
    expandNode,
    searchAndAddNodes,
    removeNode,
    removeEdge,
    addVirtualNode,
    createVirtualNode,
    createEdge,
    deleteVirtualNode,
    deleteEdge,
    refresh,
    getGraphData,
    addNodeToGraphData,
    addEdgeToGraphData,
    removeNodeFromGraphData,
  };
}
