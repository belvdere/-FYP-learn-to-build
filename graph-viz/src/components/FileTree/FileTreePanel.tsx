// File tree panel component

import { useState, useCallback, memo } from 'react';
import { TreeNode } from '../../types/fileTree';
import './FileTreePanel.css';

interface FileTreePanelProps {
  treeData: TreeNode[];
  expandedNodes: Set<string>;
  onToggle: (nodeId: string) => void;
  onNodeClick: (node: TreeNode) => void;
  onNodeDragStart?: (node: TreeNode) => void;
  onAddToCanvas?: (node: TreeNode) => void;
}

export function FileTreePanel({
  treeData,
  expandedNodes,
  onToggle,
  onNodeClick,
  onNodeDragStart,
  onAddToCanvas,
}: FileTreePanelProps) {
  const [searchQuery, setSearchQuery] = useState('');

  const filterTree = useCallback((nodes: TreeNode[], query: string): TreeNode[] => {
    if (!query) return nodes;

    return nodes.reduce<TreeNode[]>((acc, node) => {
      const matches = node.name.toLowerCase().includes(query.toLowerCase());
      const filteredChildren = node.children ? filterTree(node.children, query) : [];

      if (matches || filteredChildren.length > 0) {
        acc.push({
          ...node,
          children: filteredChildren.length > 0 ? filteredChildren : node.children,
        });
      }

      return acc;
    }, []);
  }, []);

  const filteredTree = filterTree(treeData, searchQuery);

  return (
    <div className="file-tree-panel">
      <div className="file-tree-search">
        <input
          type="text"
          placeholder="Search..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="search-input"
        />
      </div>
      <div className="file-tree-content">
        {filteredTree.length === 0 ? (
          <div className="file-tree-empty">
            {searchQuery ? 'No matches found' : 'No files loaded. Click "Load All" to load data.'}
          </div>
        ) : (
          filteredTree.map(node => (
            <TreeNodeComponent
              key={node.id}
              node={node}
              level={0}
              expandedNodes={expandedNodes}
              onToggle={onToggle}
              onClick={onNodeClick}
              onDragStart={onNodeDragStart}
              onAddToCanvas={onAddToCanvas}
            />
          ))
        )}
      </div>
    </div>
  );
}

interface TreeNodeComponentProps {
  node: TreeNode;
  level: number;
  expandedNodes: Set<string>;
  onToggle: (nodeId: string) => void;
  onClick: (node: TreeNode) => void;
  onDragStart?: (node: TreeNode) => void;
  onAddToCanvas?: (node: TreeNode) => void;
}

// Memoize for performance with large trees
const TreeNodeComponent = memo(function TreeNodeComponent({
  node,
  level,
  expandedNodes,
  onToggle,
  onClick,
  onDragStart,
  onAddToCanvas,
}: TreeNodeComponentProps) {
  const hasChildren = node.children && node.children.length > 0;
  const expanded = expandedNodes.has(node.id);
  const indent = level * 16;
  const [showAddButton, setShowAddButton] = useState(false);

  const getIcon = () => {
    switch (node.type) {
      case 'directory': return expanded ? '📂' : '📁';
      case 'file':      return '📄';
      case 'method':    return '⚙️';
      default:          return '•';
    }
  };

  const handleClick = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    if (hasChildren) onToggle(node.id);
    onClick(node);
  }, [hasChildren, node, onToggle, onClick]);

  const handleDragStart = useCallback((e: React.DragEvent) => {
    if (onDragStart) {
      onDragStart(node);
      e.dataTransfer.effectAllowed = 'copy';
      e.dataTransfer.setData('application/json', JSON.stringify(node));
    }
  }, [node, onDragStart]);

  const [copied, setCopied] = useState(false);
  const handleCopyPath = useCallback((e: React.MouseEvent) => {
    e.stopPropagation();
    const path = node.type === 'directory'
      ? node.path
      : node.path.substring(0, node.path.lastIndexOf('/'));
    navigator.clipboard.writeText(path || '').then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    });
  }, [node]);

  const canCopyPath = node.type === 'directory' || node.type === 'file';

  return (
    <div className="tree-node-container">
      <div
        className={`tree-node ${node.type} ${node.isVirtual ? 'virtual' : ''}`}
        style={{ paddingLeft: `${indent}px` }}
        onClick={handleClick}
        onMouseEnter={() => setShowAddButton(true)}
        onMouseLeave={() => setShowAddButton(false)}
        draggable={node.type === 'method'}
        onDragStart={handleDragStart}
      >
        {hasChildren && (
          <span className="expand-icon">{expanded ? '▼' : '▶'}</span>
        )}
        {!hasChildren && <span className="expand-icon-spacer" />}
        <span className="node-icon">{getIcon()}</span>
        <span className="node-label" title={node.path}>{node.name}</span>
        {node.isVirtual && <span className="virtual-badge">V</span>}
        {canCopyPath && showAddButton && (
          <button
            className="add-node-btn"
            onClick={handleCopyPath}
            title="Copy directory path"
          >
            {copied ? '✓' : '⧉'}
          </button>
        )}
        {showAddButton && onAddToCanvas && !node.isVirtual && (
          <button
            className="add-node-btn"
            onClick={(e) => { e.stopPropagation(); onAddToCanvas!(node); }}
            title="Add to canvas"
            style={{ marginLeft: 2 }}
          >
            ⊕
          </button>
        )}
      </div>
      {expanded && hasChildren && (
        <div className="tree-node-children">
          {node.children!.map(child => (
            <TreeNodeComponent
              key={child.id}
              node={child}
              level={level + 1}
              expandedNodes={expandedNodes}
              onToggle={onToggle}
              onClick={onClick}
              onDragStart={onDragStart}
              onAddToCanvas={onAddToCanvas}
            />
          ))}
        </div>
      )}
    </div>
  );
});
