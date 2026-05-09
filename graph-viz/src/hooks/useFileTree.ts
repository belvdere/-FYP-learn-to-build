// Custom hook for file tree state management

import { useState, useCallback, useMemo } from 'react';
import { TreeNode } from '../types/fileTree';
import { GraphData } from '../types/graph';

export function useFileTree(graphData: GraphData) {
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());

  // Build tree structure from graph data using useMemo for better reactivity
  // Dependencies: nodes array length and a hash of node IDs to detect changes
  const nodesHash = useMemo(() => {
    return graphData.nodes.map(n => n.id).sort().join(',');
  }, [graphData.nodes]);

  const treeData = useMemo(() => {
    console.log('[useFileTree] Rebuilding tree, nodes:', graphData.nodes.length);
    const tree = buildTreeFromGraphData(graphData);
    return compactSingleChildDirs(tree);
  }, [graphData.nodes, nodesHash]);

  const toggleNode = useCallback((nodeId: string) => {
    setExpandedNodes(prev => {
      const next = new Set(prev);
      if (next.has(nodeId)) {
        next.delete(nodeId);
      } else {
        next.add(nodeId);
      }
      return next;
    });
  }, []);

  const expandNode = useCallback((nodeId: string) => {
    setExpandedNodes(prev => new Set(prev).add(nodeId));
  }, []);

  const collapseNode = useCallback((nodeId: string) => {
    setExpandedNodes(prev => {
      const next = new Set(prev);
      next.delete(nodeId);
      return next;
    });
  }, []);

  const expandAll = useCallback(() => {
    const allIds = new Set<string>();
    const collectIds = (nodes: TreeNode[]) => {
      nodes.forEach(node => {
        if (node.type === 'directory' || node.type === 'file') {
          allIds.add(node.id);
        }
        if (node.children) {
          collectIds(node.children);
        }
      });
    };
    collectIds(treeData);
    setExpandedNodes(allIds);
  }, [treeData]);

  const collapseAll = useCallback(() => {
    setExpandedNodes(new Set());
  }, []);

  return {
    treeData,
    expandedNodes,
    toggleNode,
    expandNode,
    collapseNode,
    expandAll,
    collapseAll,
  };
}

// Helper function to build tree structure from nodes' filePath
function buildTreeFromGraphData(graphData: GraphData): TreeNode[] {
  if (graphData.nodes.length === 0) {
    return [];
  }

  // Maps to track directories and files we create
  const dirMap = new Map<string, TreeNode>();
  const fileMap = new Map<string, TreeNode>();

  // Process each node and build directory/file structure from filePath
  graphData.nodes.forEach(node => {
    const filePath = node.filePath;
    if (!filePath) return;

    // Split path into segments
    const segments = filePath.split('/').filter(s => s.length > 0);
    if (segments.length === 0) return;

    const fileName = segments[segments.length - 1];
    const dirSegments = segments.slice(0, -1);

    // Build directory hierarchy
    let currentPath = '';
    let lastDirPath = '';

    for (const segment of dirSegments) {
      currentPath = currentPath ? `${currentPath}/${segment}` : segment;
      
      if (!dirMap.has(currentPath)) {
        const dirNode: TreeNode = {
          id: `dir:${currentPath}`,
          name: segment,
          type: 'directory',
          path: currentPath,
          children: [],
          isVirtual: false,
        };
        dirMap.set(currentPath, dirNode);

        // Add to parent directory
        if (lastDirPath) {
          const parentDir = dirMap.get(lastDirPath);
          if (parentDir && parentDir.children) {
            parentDir.children.push(dirNode);
          }
        }
      }

      lastDirPath = currentPath;
    }

    // Create or get file node
    if (!fileMap.has(filePath)) {
      const fileNode: TreeNode = {
        id: `file:${filePath}`,
        name: fileName,
        type: 'file',
        path: filePath,
        children: [],
        isVirtual: false,
      };
      fileMap.set(filePath, fileNode);

      // Add to parent directory if not at root level
      if (lastDirPath) {
        const parentDir = dirMap.get(lastDirPath);
        if (parentDir && parentDir.children) {
          parentDir.children.push(fileNode);
        }
      }
    }

    // Add method node to file
    const fileNode = fileMap.get(filePath)!;
    const methodNode: TreeNode = {
      id: node.id,
      name: node.label,
      type: 'method',
      path: filePath,
      line: node.line,
      isVirtual: node.isVirtual,
      parentId: fileNode.id,
    };
    fileNode.children!.push(methodNode);
  });

  // Find root directories (those not in any parent)
  const rootDirs: TreeNode[] = [];

  dirMap.forEach((dir, path) => {
    // Check if this directory has a parent
    const segments = path.split('/');
    if (segments.length === 1) {
      // Top-level directory
      rootDirs.push(dir);
    }
  });

  // Also add files that are at root level (no directory)
  fileMap.forEach((file, path) => {
    const segments = path.split('/').filter(s => s.length > 0);
    if (segments.length === 1) {
      // File at root level
      rootDirs.push(file);
    }
  });

  // Sort children recursively
  const sortChildren = (nodes: TreeNode[]) => {
    nodes.sort((a, b) => {
      // Directories first, then files, then methods
      const typeOrder = { directory: 0, file: 1, method: 2 };
      const aOrder = typeOrder[a.type as keyof typeof typeOrder] ?? 3;
      const bOrder = typeOrder[b.type as keyof typeof typeOrder] ?? 3;
      if (aOrder !== bOrder) return aOrder - bOrder;
      return a.name.localeCompare(b.name);
    });
    nodes.forEach(node => {
      if (node.children && node.children.length > 0) {
        sortChildren(node.children);
      }
    });
  };

  sortChildren(rootDirs);

  return rootDirs;
}

/**
 * Compact single-child directory chains into one node.
 * e.g., src → main → java → com → example → service (each with one child dir)
 * becomes: "src/main/java/com/example/service" as a single directory node.
 *
 * Stops merging when a directory has:
 *  - Multiple children, OR
 *  - A non-directory child (file or method)
 */
function compactSingleChildDirs(nodes: TreeNode[]): TreeNode[] {
  return nodes.map(node => {
    if (node.type !== 'directory' || !node.children || node.children.length === 0) {
      return node;
    }

    // Recursively compact children first
    let compacted: TreeNode = {
      ...node,
      children: compactSingleChildDirs(node.children),
    };

    // Merge downward while the directory has exactly one child that is also a directory
    while (
      compacted.children &&
      compacted.children.length === 1 &&
      compacted.children[0].type === 'directory'
    ) {
      const onlyChild = compacted.children[0];
      compacted = {
        ...compacted,
        name: `${compacted.name}/${onlyChild.name}`,
        path: onlyChild.path,
        id: onlyChild.id,                    // Use the deeper node's ID for expansion state
        children: onlyChild.children || [],
        isVirtual: compacted.isVirtual || onlyChild.isVirtual,
      };
    }

    return compacted;
  });
}

