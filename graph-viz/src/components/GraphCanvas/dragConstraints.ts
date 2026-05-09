import type cytoscape from 'cytoscape';

export interface ApplyChildDragConstraintArgs {
  node: cytoscape.NodeSingular;
  draggingParentId: string | null;
  isContainerNode: (node: cytoscape.NodeSingular) => boolean;
  isDescendantOf: (node: cytoscape.NodeSingular, ancestorId: string) => boolean;
  clampChildInsideParent: (child: cytoscape.NodeSingular, parent: cytoscape.NodeSingular) => void;
}

/**
 * Applies child-drag constraints for compound nodes.
 * Important invariant: this function never repositions the parent; only the dragged child is clamped.
 */
export function applyChildDragConstraint({
  node,
  draggingParentId,
  isContainerNode,
  isDescendantOf,
  clampChildInsideParent,
}: ApplyChildDragConstraintArgs): void {
  if (!node.isNode()) return;
  if (node.isParent()) return;

  const parent = node.parent()[0] as cytoscape.NodeSingular | undefined;
  if (!parent || !isContainerNode(parent)) return;

  if (draggingParentId && isDescendantOf(node, draggingParentId)) return;
  clampChildInsideParent(node, parent);
}

