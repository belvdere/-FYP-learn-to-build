import { describe, expect, it, vi } from 'vitest';
import type cytoscape from 'cytoscape';
import { applyChildDragConstraint } from './dragConstraints';

function makeNode(overrides: Partial<cytoscape.NodeSingular>): cytoscape.NodeSingular {
  const base = {
    isNode: () => true,
    isParent: () => false,
    parent: () => [] as unknown as cytoscape.NodeCollection,
  };
  return { ...base, ...overrides } as unknown as cytoscape.NodeSingular;
}

describe('applyChildDragConstraint', () => {
  it('clamps only the dragged child and does not move parent', () => {
    const parentPosition = vi.fn();
    const parent = makeNode({
      isParent: () => true,
      position: parentPosition as unknown as cytoscape.NodeSingular['position'],
    });
    const child = makeNode({
      parent: () => [parent] as unknown as cytoscape.NodeCollection,
    });

    const clampChildInsideParent = vi.fn();

    applyChildDragConstraint({
      node: child,
      draggingParentId: null,
      isContainerNode: () => true,
      isDescendantOf: () => false,
      clampChildInsideParent,
    });

    expect(clampChildInsideParent).toHaveBeenCalledTimes(1);
    expect(clampChildInsideParent).toHaveBeenCalledWith(child, parent);
    expect(parentPosition).not.toHaveBeenCalled();
  });

  it('skips clamping descendants while a parent drag is active', () => {
    const parent = makeNode({
      isParent: () => true,
    });
    const child = makeNode({
      parent: () => [parent] as unknown as cytoscape.NodeCollection,
    });

    const clampChildInsideParent = vi.fn();

    applyChildDragConstraint({
      node: child,
      draggingParentId: 'dir-1',
      isContainerNode: () => true,
      isDescendantOf: () => true,
      clampChildInsideParent,
    });

    expect(clampChildInsideParent).not.toHaveBeenCalled();
  });
});

