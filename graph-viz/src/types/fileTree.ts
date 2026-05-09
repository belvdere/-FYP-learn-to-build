// File tree types

export interface TreeNode {
  id: string;
  name: string;
  type: 'directory' | 'file' | 'method';
  path: string;
  children?: TreeNode[];
  isVirtual?: boolean;
  isExpanded?: boolean;
  line?: number;
  parentId?: string;
}

export interface FileTreeData {
  root: TreeNode[];
}

