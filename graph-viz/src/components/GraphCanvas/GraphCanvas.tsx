// Main graph canvas component using Cytoscape.js
// Features: conditional parent-child containment, resize handles, layer visibility toggles, edge handles

import { useEffect, useRef, useState, useCallback } from 'react';
import cytoscape, { Core, EventObject } from 'cytoscape';
// @ts-ignore - dagre doesn't have proper types
import dagre from 'cytoscape-dagre';
// @ts-ignore - edgehandles types via @types/cytoscape-edgehandles
import edgehandles from 'cytoscape-edgehandles';
import { GraphData, GraphNode, GraphEdge, CytoscapeNodeType } from '../../types/graph';
import { buildCytoscapeElements } from '../../utils/graphBuilder';
import { cytoscapeStyles } from '../../utils/cytoscapeStyles';
import { dagreLayout } from '../../utils/layoutConfig';
import { GraphLegend } from '../GraphLegend/GraphLegend';
import { PlacementTool } from './ShapePalette';
import { applyChildDragConstraint } from './dragConstraints';
import './GraphCanvas.css';

// Register plugins once at module level
cytoscape.use(dagre);
cytoscape.use(edgehandles);

// Layer type groups (for visibility toggles)
type LayerGroup = 'directory' | 'file' | 'method';
const LAYER_TYPE_MAP: Record<LayerGroup, CytoscapeNodeType[]> = {
  directory: ['directory', 'virtual-directory'],
  file: ['file', 'virtual-class'],
  method: ['method', 'field', 'virtual'],
};
const LAYER_LABELS: Record<LayerGroup, string> = {
  directory: 'Dir',
  file: 'File',
  method: 'Method',
};

interface GraphCanvasProps {
  graphData: GraphData;
  placementMode: PlacementTool | null;
  selectedNodeId?: string;
  /** Optional allow-list of node IDs that can participate in persisted edge creation. */
  connectableNodeIds?: ReadonlySet<string>;
  /** Ref to a Map of nodeId → position (graph coords). GraphCanvas applies and removes entries. */
  pendingNodePositionsRef?: React.MutableRefObject<Map<string, { x: number; y: number }>>;
  onNodeClick?: (node: GraphNode) => void;
  onNodeDoubleClick?: (node: GraphNode) => void;
  onNodeDelete?: (node: GraphNode) => void;
  onEdgeClick?: (edge: GraphEdge) => void;
  onBackgroundClick?: () => void;
  onPlaceVirtualNode?: (type: PlacementTool, position: { x: number; y: number }) => void;
  onNodeRightClickAddParent?: (node: GraphNode) => void;
  onEdgeDrawn?: (sourceId: string, targetId: string) => void;
}

interface CallbackRefs {
  onNodeClick?: (node: GraphNode) => void;
  onNodeDoubleClick?: (node: GraphNode) => void;
  onNodeDelete?: (node: GraphNode) => void;
  onEdgeClick?: (edge: GraphEdge) => void;
  onBackgroundClick?: () => void;
  onPlaceVirtualNode?: (type: PlacementTool, position: { x: number; y: number }) => void;
  onNodeRightClickAddParent?: (node: GraphNode) => void;
  onEdgeDrawn?: (sourceId: string, targetId: string) => void;
  placementMode: PlacementTool | null;
}

// Corner handle sizing & positioning
const HANDLE_SIZE = 9;
const HANDLE_OFFSET = -Math.floor(HANDLE_SIZE / 2);

type ResizeCorner = 'nw' | 'ne' | 'se' | 'sw';

const CORNER_CURSOR: Record<ResizeCorner, string> = {
  nw: 'nw-resize', ne: 'ne-resize', se: 'se-resize', sw: 'sw-resize',
};

const CORNER_STYLE: Record<ResizeCorner, React.CSSProperties> = {
  nw: { top: HANDLE_OFFSET, left: HANDLE_OFFSET },
  ne: { top: HANDLE_OFFSET, right: HANDLE_OFFSET },
  se: { bottom: HANDLE_OFFSET, right: HANDLE_OFFSET },
  sw: { bottom: HANDLE_OFFSET, left: HANDLE_OFFSET },
};

interface ResizeDragState {
  nodeId: string;
  corner: ResizeCorner;
  startMouseX: number;
  startMouseY: number;
  startModelW: number;
  startModelH: number;
  startModelCX: number;
  startModelCY: number;
}

const CONTAINER_TYPES = new Set<string>(['directory', 'file', 'virtual-directory', 'virtual-class']);
const PARENT_PADDING_X = 18;
const PARENT_PADDING_Y = 12;
const PARENT_HEADER_HEIGHT = 26;
const SLOT_GAP_X = 24;
const SLOT_GAP_Y = 20;

function clamp(value: number, min: number, max: number): number {
  if (min > max) return value;
  return Math.min(Math.max(value, min), max);
}

function getNodeMinDimension(node: cytoscape.NodeSingular, prop: 'min-width' | 'min-height'): number {
  const raw = String(node.style(prop));
  const parsed = Number.parseFloat(raw);
  if (Number.isFinite(parsed) && parsed > 0) return parsed;
  return prop === 'min-width' ? node.width() : node.height();
}

function setNodeMinSize(node: cytoscape.NodeSingular, minW: number, minH: number): void {
  node.style({
    'min-width': Math.max(1, minW),
    'min-height': Math.max(1, minH),
  });
}

function isContainerNode(node: cytoscape.NodeSingular): boolean {
  return CONTAINER_TYPES.has(String(node.data('type') || ''));
}

function getParentInteriorBounds(parent: cytoscape.NodeSingular, childW: number, childH: number) {
  const pw = parent.width();
  const ph = parent.height();
  const center = parent.position();

  return {
    left: center.x - pw / 2 + PARENT_PADDING_X + childW / 2,
    right: center.x + pw / 2 - PARENT_PADDING_X - childW / 2,
    top: center.y - ph / 2 + PARENT_HEADER_HEIGHT + PARENT_PADDING_Y + childH / 2,
    bottom: center.y + ph / 2 - PARENT_PADDING_Y - childH / 2,
  };
}

function ensureParentCanHostChild(parent: cytoscape.NodeSingular, child: cytoscape.NodeSingular): void {
  const requiredW = child.width() + 2 * PARENT_PADDING_X + 4;
  const requiredH = child.height() + PARENT_HEADER_HEIGHT + 2 * PARENT_PADDING_Y + 4;
  const minW = getNodeMinDimension(parent, 'min-width');
  const minH = getNodeMinDimension(parent, 'min-height');
  if (requiredW > minW || requiredH > minH) {
    setNodeMinSize(parent, Math.max(minW, requiredW), Math.max(minH, requiredH));
  }
}

function fitParentToChildren(parent: cytoscape.NodeSingular): void {
  const parentPos = parent.position();
  let requiredW = getNodeMinDimension(parent, 'min-width');
  let requiredH = getNodeMinDimension(parent, 'min-height');

  const userMinW = Number(parent.data('userMinW'));
  const userMinH = Number(parent.data('userMinH'));
  if (Number.isFinite(userMinW) && userMinW > 0) requiredW = Math.max(requiredW, userMinW);
  if (Number.isFinite(userMinH) && userMinH > 0) requiredH = Math.max(requiredH, userMinH);

  parent.children().forEach(child => {
    if (!child.isNode()) return;
    const node = child as cytoscape.NodeSingular;
    const cp = node.position();
    const cw = node.width();
    const ch = node.height();

    const halfWNeeded = Math.abs(cp.x - parentPos.x) + cw / 2 + PARENT_PADDING_X + 2;
    const topHalfNeeded = (parentPos.y - (cp.y - ch / 2)) + PARENT_HEADER_HEIGHT + PARENT_PADDING_Y + 2;
    const bottomHalfNeeded = ((cp.y + ch / 2) - parentPos.y) + PARENT_PADDING_Y + 2;
    const halfHNeeded = Math.max(topHalfNeeded, bottomHalfNeeded);

    requiredW = Math.max(requiredW, halfWNeeded * 2);
    requiredH = Math.max(requiredH, halfHNeeded * 2);
  });

  setNodeMinSize(parent, requiredW, requiredH);
}

function ensureParentContainsChild(parent: cytoscape.NodeSingular, child: cytoscape.NodeSingular): void {
  ensureParentCanHostChild(parent, child);
  fitParentToChildren(parent);
  clampChildInsideParent(child, parent);
}

function clampChildInsideParent(child: cytoscape.NodeSingular, parent: cytoscape.NodeSingular): void {
  ensureParentCanHostChild(parent, child);
  const bounds = getParentInteriorBounds(parent, child.width(), child.height());
  const p = child.position();
  child.position({
    x: clamp(p.x, bounds.left, bounds.right),
    y: clamp(p.y, bounds.top, bounds.bottom),
  });
}

function rectsOverlap(
  a: { x1: number; x2: number; y1: number; y2: number },
  b: { x1: number; x2: number; y1: number; y2: number }
): boolean {
  return !(a.x2 < b.x1 || b.x2 < a.x1 || a.y2 < b.y1 || b.y2 < a.y1);
}

function placeChildInParentSlot(child: cytoscape.NodeSingular, parent: cytoscape.NodeSingular): void {
  ensureParentCanHostChild(parent, child);

  const cw = child.width();
  const ch = child.height();
  const bounds = getParentInteriorBounds(parent, cw, ch);
  const occupied: Array<{ x1: number; x2: number; y1: number; y2: number }> = [];
  parent.children().forEach(ele => {
    if (!ele.isNode() || ele.id() === child.id()) return;
    const n = ele as cytoscape.NodeSingular;
    const p = n.position();
    const nw = n.width();
    const nh = n.height();
    occupied.push({
      x1: p.x - nw / 2 - 8,
      x2: p.x + nw / 2 + 8,
      y1: p.y - nh / 2 - 8,
      y2: p.y + nh / 2 + 8,
    });
  });

  const stepX = cw + SLOT_GAP_X;
  const stepY = ch + SLOT_GAP_Y;
  for (let y = bounds.top; y <= bounds.bottom; y += stepY) {
    for (let x = bounds.left; x <= bounds.right; x += stepX) {
      const candidate = {
        x1: x - cw / 2,
        x2: x + cw / 2,
        y1: y - ch / 2,
        y2: y + ch / 2,
      };
      const intersects = occupied.some(r => rectsOverlap(candidate, r));
      if (!intersects) {
        child.position({ x, y });
        ensureParentContainsChild(parent, child);
        return;
      }
    }
  }

  // Fallback: place at parent center and enforce bounds.
  const parentPos = parent.position();
  child.position({ x: parentPos.x, y: parentPos.y });
  ensureParentContainsChild(parent, child);
}

function ensureParentContainsAllChildren(parent: cytoscape.NodeSingular): void {
  fitParentToChildren(parent);
  parent.children().forEach(child => {
    if (!child.isNode()) return;
    clampChildInsideParent(child as cytoscape.NodeSingular, parent);
  });
  fitParentToChildren(parent);
}

function getNodeDepth(node: cytoscape.NodeSingular): number {
  let depth = 0;
  let current = node.parent()[0] as cytoscape.NodeSingular | undefined;
  while (current) {
    depth += 1;
    current = current.parent()[0] as cytoscape.NodeSingular | undefined;
  }
  return depth;
}

function isDescendantOf(node: cytoscape.NodeSingular, ancestorId: string): boolean {
  let current = node.parent()[0] as cytoscape.NodeSingular | undefined;
  while (current) {
    if (current.id() === ancestorId) return true;
    current = current.parent()[0] as cytoscape.NodeSingular | undefined;
  }
  return false;
}

function getViewportCenter(cy: Core): cytoscape.Position {
  const extent = cy.extent();
  return { x: (extent.x1 + extent.x2) / 2, y: (extent.y1 + extent.y2) / 2 };
}

function getNodePositionByID(cy: Core, nodeId: string): cytoscape.Position | null {
  const node = cy.getElementById(nodeId) as cytoscape.NodeSingular;
  if (!node.length || !node.isNode()) return null;
  return { ...node.position() };
}

function placeTopLevelNodesAroundAnchor(cy: Core, nodeIDs: string[], anchor: cytoscape.Position): void {
  if (nodeIDs.length === 0) return;
  const baseRadius = 180;
  const ringStep = 130;
  const perRing = 10;
  nodeIDs.forEach((id, idx) => {
    const node = cy.getElementById(id) as cytoscape.NodeSingular;
    if (!node.length || !node.isNode()) return;
    const ring = Math.floor(idx / perRing);
    const indexInRing = idx % perRing;
    const countInRing = Math.min(perRing, nodeIDs.length - ring * perRing);
    const angle = (2 * Math.PI * indexInRing) / Math.max(1, countInRing) - Math.PI / 2;
    const radius = baseRadius + ring * ringStep;
    node.position({
      x: anchor.x + radius * Math.cos(angle),
      y: anchor.y + radius * Math.sin(angle),
    });
  });
}

function enforceAllCompoundConstraints(cy: Core): void {
  const parents = cy
    .nodes()
    .filter(node => node.isParent() && isContainerNode(node as cytoscape.NodeSingular))
    .toArray()
    .sort((a, b) => getNodeDepth(b as cytoscape.NodeSingular) - getNodeDepth(a as cytoscape.NodeSingular));

  parents.forEach(parent => {
    ensureParentContainsAllChildren(parent as cytoscape.NodeSingular);
  });
}

export function GraphCanvas({
  graphData,
  placementMode,
  selectedNodeId,
  connectableNodeIds,
  pendingNodePositionsRef,
  onNodeClick,
  onNodeDoubleClick,
  onNodeDelete,
  onEdgeClick,
  onBackgroundClick,
  onPlaceVirtualNode,
  onNodeRightClickAddParent,
  onEdgeDrawn,
}: GraphCanvasProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<Core | null>(null);
  const ehRef = useRef<any>(null);

  const [layoutType, setLayoutType] = useState<'dagre' | 'cose' | 'grid'>('dagre');
  const [showLegend, setShowLegend] = useState(false);
  const [edgeDrawMode, setEdgeDrawMode] = useState(false);
  const connectableNodeIdsRef = useRef<ReadonlySet<string> | undefined>(connectableNodeIds);
  useEffect(() => {
    connectableNodeIdsRef.current = connectableNodeIds;
  }, [connectableNodeIds]);

  // Layer visibility state + ref (ref keeps graphData effect from going stale)
  const [layerVisibility, setLayerVisibility] = useState<Record<LayerGroup, boolean>>({
    directory: true, file: true, method: true,
  });
  const layerVisibilityRef = useRef(layerVisibility);
  useEffect(() => { layerVisibilityRef.current = layerVisibility; }, [layerVisibility]);

  // Resize overlay state — tracks selected node bounding box in DOM/canvas coords
  const [resizeBB, setResizeBB] = useState<{ x1: number; y1: number; w: number; h: number } | null>(null);
  const selectedNodeIdRef = useRef<string | null>(null);
  const resizeDragRef = useRef<ResizeDragState | null>(null);

  // Track element IDs currently in Cytoscape for diffing
  const prevElementIdsRef = useRef<{ nodes: Set<string>; edges: Set<string> }>({
    nodes: new Set(), edges: new Set(),
  });

  // Action and interaction anchors for deterministic delta placement.
  const pendingActionAnchorRef = useRef<{ nodeId: string; pos: cytoscape.Position } | null>(null);
  const lastInteractionAnchorRef = useRef<{ nodeId: string; pos: cytoscape.Position } | null>(null);
  const draggingParentIdRef = useRef<string | null>(null);

  // Stable callbacks ref — avoids recreating the Cytoscape instance on prop changes
  const callbacksRef = useRef<CallbackRefs>({
    onNodeClick, onNodeDoubleClick, onNodeDelete, onEdgeClick,
    onBackgroundClick, onPlaceVirtualNode, onNodeRightClickAddParent, onEdgeDrawn,
    placementMode,
  });
  useEffect(() => {
    callbacksRef.current = {
      onNodeClick, onNodeDoubleClick, onNodeDelete, onEdgeClick,
      onBackgroundClick, onPlaceVirtualNode, onNodeRightClickAddParent, onEdgeDrawn,
      placementMode,
    };
  }, [onNodeClick, onNodeDoubleClick, onNodeDelete, onEdgeClick, onBackgroundClick,
      onPlaceVirtualNode, onNodeRightClickAddParent, onEdgeDrawn, placementMode]);

  // Recompute resize overlay BB for the currently selected node
  const refreshResizeBB = useCallback(() => {
    const cy = cyRef.current;
    const id = selectedNodeIdRef.current;
    if (!cy || !id) { setResizeBB(null); return; }
    const node = cy.getElementById(id);
    if (!node.length || !node.isNode()) { setResizeBB(null); return; }
    const bb = node.renderedBoundingBox({ includeLabels: false, includeOverlays: false });
    setResizeBB({ x1: bb.x1, y1: bb.y1, w: bb.w, h: bb.h });
  }, []);

  // Apply layer class visibility to all Cytoscape nodes
  const syncLayerVisibility = useCallback((cy: Core, visibility: Record<LayerGroup, boolean>) => {
    (Object.entries(LAYER_TYPE_MAP) as [LayerGroup, CytoscapeNodeType[]][]).forEach(([group, types]) => {
      types.forEach(type => {
        const nodes = cy.nodes(`[type="${type}"]`);
        if (visibility[group]) nodes.removeClass('layer-hidden');
        else nodes.addClass('layer-hidden');
      });
    });
  }, []);

  // Initialize Cytoscape once
  useEffect(() => {
    if (!containerRef.current) return;

    const cy = cytoscape({
      container: containerRef.current,
      elements: [],
      style: cytoscapeStyles,
      layout: { name: 'preset' },
      minZoom: 0.1,
      maxZoom: 3,
      wheelSensitivity: 0.2,
    });
    cyRef.current = cy;

    // Edge handles
    const eh = (cy as any).edgehandles({
      canConnect: (s: any, t: any) => {
        if (s.id() === t.id()) return false;
        const ids = connectableNodeIdsRef.current;
        if (!ids) return true;
        return ids.has(s.id()) && ids.has(t.id());
      },
      snap: false,
      noEdgeEventsInDraw: true,
    });
    ehRef.current = eh;

    cy.on('ehcomplete', (_evt: any, sourceNode: any, targetNode: any, addedEdge: any) => {
      if (addedEdge) addedEdge.remove(); // remove temp Cytoscape edge; real edge created via API
      callbacksRef.current.onEdgeDrawn?.(sourceNode.id(), targetNode.id());
    });

    // Bug fix: tap fires for BOTH clicks in a double-click sequence.
    // Delay single-click by 250ms so dbltap can cancel it first.
    let tapTimer: ReturnType<typeof setTimeout> | null = null;

    cy.on('tap', 'node', (evt: EventObject) => {
      const nodeData = evt.target.data('nodeData') as GraphNode | undefined;
      if (!nodeData) return;
      lastInteractionAnchorRef.current = { nodeId: nodeData.id, pos: { ...evt.target.position() } };
      if (tapTimer) { clearTimeout(tapTimer); tapTimer = null; }
      tapTimer = setTimeout(() => {
        tapTimer = null;
        callbacksRef.current.onNodeClick?.(nodeData);
      }, 220);
    });

    // Double-click — cancel pending single-click first, then expand neighborhood
    cy.on('dbltap', 'node', (evt: EventObject) => {
      if (tapTimer) { clearTimeout(tapTimer); tapTimer = null; }
      const nodeData = evt.target.data('nodeData') as GraphNode | undefined;
      if (!nodeData) return;
      const pos = { ...evt.target.position() };
      pendingActionAnchorRef.current = { nodeId: nodeData.id, pos };
      lastInteractionAnchorRef.current = { nodeId: nodeData.id, pos };
      callbacksRef.current.onNodeDoubleClick?.(nodeData);
    });

    // Reusable helper: sync edge highlights to currently-selected nodes
    const syncEdgeHighlights = () => {
      cy.edges().removeClass('edge-highlighted');
      cy.nodes(':selected').connectedEdges().addClass('edge-highlighted');
    };

    // select — update resize overlay + edge highlights
    cy.on('select', 'node', (evt: EventObject) => {
      selectedNodeIdRef.current = evt.target.id();
      refreshResizeBB();
      syncEdgeHighlights();
    });

    // tap on node also refreshes resize overlay for already-selected nodes
    // (select doesn't re-fire if the node is already selected)
    cy.on('tap', 'node', (evt: EventObject) => {
      if (evt.target.selected()) refreshResizeBB();
    });

    // unselect — sync overlay and edge highlights in one pass
    cy.on('unselect', 'node', () => {
      syncEdgeHighlights();
      const remaining = cy.nodes(':selected');
      if (remaining.length === 0) {
        selectedNodeIdRef.current = null;
        setResizeBB(null);
      } else {
        // Keep overlay on the most-recently-selected remaining node
        selectedNodeIdRef.current = remaining.last().id();
        refreshResizeBB();
      }
    });

    // remove — node deleted from graph; clear overlay if it was selected
    cy.on('remove', 'node', (evt: EventObject) => {
      if (evt.target.id() === selectedNodeIdRef.current) {
        selectedNodeIdRef.current = null;
        setResizeBB(null);
      }
    });

    // pan / zoom — throttle overlay refresh via rAF to avoid 60fps setState spam
    let rafHandle: number | null = null;
    const rafRefresh = () => {
      if (rafHandle !== null) return;
      rafHandle = requestAnimationFrame(() => { rafHandle = null; refreshResizeBB(); });
    };
    cy.on('pan zoom', rafRefresh);
    cy.on('position', 'node', (evt: EventObject) => {
      if (evt.target.id() === selectedNodeIdRef.current) rafRefresh();
    });

    cy.on('grab', 'node', (evt: EventObject) => {
      const node = evt.target as cytoscape.NodeSingular;
      if (node.isParent() && isContainerNode(node)) {
        draggingParentIdRef.current = node.id();
        return;
      }
      draggingParentIdRef.current = null;
    });

    cy.on('free', 'node', (evt: EventObject) => {
      const node = evt.target as cytoscape.NodeSingular;
      if (draggingParentIdRef.current === node.id()) {
        draggingParentIdRef.current = null;
      }
    });

    // Child nodes must stay inside their visible parent.
    const clampDraggedChild = (evt: EventObject) => {
      const node = evt.target as cytoscape.NodeSingular;
      applyChildDragConstraint({
        node,
        draggingParentId: draggingParentIdRef.current,
        isContainerNode,
        isDescendantOf,
        clampChildInsideParent,
      });
    };
    cy.on('drag', 'node', clampDraggedChild);
    cy.on('dragfree', 'node', clampDraggedChild);

    // Right-click — add parent node to canvas
    cy.on('cxttap', 'node', (evt: EventObject) => {
      const nodeData = evt.target.data('nodeData') as GraphNode | undefined;
      const type = evt.target.data('type') as string;
      if (nodeData && (type === 'method' || type === 'field' || type === 'file')) {
        const pos = { ...evt.target.position() };
        lastInteractionAnchorRef.current = { nodeId: nodeData.id, pos };
        callbacksRef.current.onNodeRightClickAddParent?.(nodeData);
      }
    });

    // Edge click
    cy.on('tap', 'edge', (evt: EventObject) => {
      const edgeData = evt.target.data('edgeData') as GraphEdge | undefined;
      if (edgeData) callbacksRef.current.onEdgeClick?.(edgeData);
    });

    // Background tap — placement mode or clear selection
    cy.on('tap', (evt: EventObject) => {
      if (evt.target !== cy) return;
      const { placementMode: mode, onPlaceVirtualNode: placeFn, onBackgroundClick: bgClick } = callbacksRef.current;
      if (mode && placeFn) { placeFn(mode, evt.position); return; }
      bgClick?.();
    });

    return () => {
      // Cancel pending timer and rAF to avoid post-unmount state updates
      if (tapTimer) { clearTimeout(tapTimer); tapTimer = null; }
      if (rafHandle !== null) { cancelAnimationFrame(rafHandle); rafHandle = null; }
      draggingParentIdRef.current = null;
      cy.off('drag', 'node', clampDraggedChild);
      cy.off('dragfree', 'node', clampDraggedChild);
      cy.destroy();
      cyRef.current = null;
      ehRef.current = null;
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Delete / Backspace key — remove selected nodes from canvas
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Delete' && e.key !== 'Backspace') return;
      const tag = (document.activeElement as HTMLElement | null)?.tagName ?? '';
      if (tag === 'INPUT' || tag === 'TEXTAREA') return;
      if (!cyRef.current) return;
      cyRef.current.elements(':selected').forEach(ele => {
        if (ele.isNode()) {
          const nodeData = ele.data('nodeData') as GraphNode | undefined;
          if (nodeData) callbacksRef.current.onNodeDelete?.(nodeData);
        }
      });
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  // Toggle edge draw mode — enableDrawMode shows handles on hover over the whole node body
  // Use direct ref read (not state updater) to avoid React strict-mode double-invoke
  const edgeDrawModeRef = useRef(false);
  const handleToggleEdgeDraw = useCallback(() => {
    const eh = ehRef.current;
    if (!eh) return;
    const next = !edgeDrawModeRef.current;
    edgeDrawModeRef.current = next;
    if (next) {
      eh.enableDrawMode();
    } else {
      eh.disableDrawMode();
    }
    setEdgeDrawMode(next);
  }, []);

  // Escape key cancels edge draw mode
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && edgeDrawModeRef.current) {
        ehRef.current?.disableDrawMode();
        edgeDrawModeRef.current = false;
        setEdgeDrawMode(false);
      }
    };
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, []);

  // Crosshair cursor during placement mode
  useEffect(() => {
    const container = cyRef.current?.container();
    if (container) container.style.cursor = placementMode ? 'crosshair' : 'default';
  }, [placementMode]);

  // Layer visibility sync (reads from state, not stale closure)
  useEffect(() => {
    if (!cyRef.current) return;
    syncLayerVisibility(cyRef.current, layerVisibility);
  }, [layerVisibility, syncLayerVisibility]);

  // Graph diff — add/remove elements incrementally, no full nuke
  useEffect(() => {
    if (!cyRef.current) return;
    const cy = cyRef.current;
    const newElements = buildCytoscapeElements(graphData);

    const newNodeIds = new Set<string>();
    const newEdgeIds = new Set<string>();
    const newElementMap = new Map<string, any>();

    for (const el of newElements) {
      const id = el.data.id;
      if ('source' in el.data) newEdgeIds.add(id);
      else newNodeIds.add(id);
      newElementMap.set(id, el);
    }

    const prev = prevElementIdsRef.current;
    const nodesToAdd = [...newNodeIds].filter(id => !prev.nodes.has(id));
    const nodesToRemove = [...prev.nodes].filter(id => !newNodeIds.has(id));
    const edgesToAdd = [...newEdgeIds].filter(id => !prev.edges.has(id));
    const edgesToRemove = [...prev.edges].filter(id => !newEdgeIds.has(id));
    // Nodes that exist in both old and new — may have updated label / annotation
    const unchangedNodeIds = [...newNodeIds].filter(id => prev.nodes.has(id));

    const hasAdded = nodesToAdd.length > 0 || edgesToAdd.length > 0;
    const isFirstLoad = prev.nodes.size === 0 && newNodeIds.size > 0;
    const reparentedNodeIDs: string[] = [];
    const hadPendingActionAnchor = pendingActionAnchorRef.current !== null;

    if (newNodeIds.size === 0) {
      pendingActionAnchorRef.current = null;
      lastInteractionAnchorRef.current = null;
      draggingParentIdRef.current = null;
      selectedNodeIdRef.current = null;
      setResizeBB(null);
    }

    const topLevelNodeElementsToAdd = nodesToAdd
      .map(id => newElementMap.get(id))
      .filter((el: any) => el && !('source' in el.data) && !el.data.parent);
    const childNodeElementsToAdd = nodesToAdd
      .map(id => newElementMap.get(id))
      .filter((el: any) => el && !('source' in el.data) && !!el.data.parent);
    const edgeElementsToAdd = edgesToAdd
      .map(id => newElementMap.get(id))
      .filter(Boolean);

    // Batch updates:
    // 1) remove stale edges
    // 2) add top-level nodes then child nodes (parents first)
    // 3) update existing nodes (labels/annotations + parent moves)
    // 4) unparent children of removed parents (promotion)
    // 5) remove stale nodes
    // 6) add new edges
    cy.batch(() => {
      for (const id of edgesToRemove) cy.getElementById(id).remove();

      if (topLevelNodeElementsToAdd.length > 0) cy.add(topLevelNodeElementsToAdd);
      if (childNodeElementsToAdd.length > 0) cy.add(childNodeElementsToAdd);

      // Refresh label + nodeData on existing nodes so label renames and annotation
      // changes are reflected on the canvas without a full remove/re-add cycle.
      for (const id of unchangedNodeIds) {
        const newEl = newElementMap.get(id);
        if (!newEl || 'source' in newEl.data) continue;
        const cyNode = cy.getElementById(id) as cytoscape.NodeSingular;
        if (cyNode.length) {
          const oldParentNode = cyNode.parent()[0] as cytoscape.NodeSingular | undefined;
          const oldParent = oldParentNode ? oldParentNode.id() : undefined;
          const nextParent = newEl.data.parent as string | undefined;
          if (oldParent !== nextParent) {
            const keepPosition = { ...cyNode.position() };
            cyNode.move({ parent: nextParent ?? null });
            cyNode.position(keepPosition);
            if (nextParent) reparentedNodeIDs.push(id);
          }
          cyNode.data({
            label: newEl.data.label,
            nodeData: newEl.data.nodeData,
            type: newEl.data.type,
            isVirtual: newEl.data.isVirtual,
            isStaleSnapshot: newEl.data.isStaleSnapshot,
          });
        }
      }

      // Promote visible descendants to top-level before removing parent nodes.
      for (const parentID of nodesToRemove) {
        const parentNode = cy.getElementById(parentID) as cytoscape.NodeSingular;
        if (!parentNode.length || !parentNode.isNode()) continue;
        parentNode.children().forEach(child => {
          if (!child.isNode()) return;
          if (!newNodeIds.has(child.id())) return;
          const keepPosition = { ...(child as cytoscape.NodeSingular).position() };
          child.move({ parent: null });
          (child as cytoscape.NodeSingular).position(keepPosition);
        });
      }

      for (const id of nodesToRemove) cy.getElementById(id).remove();
      if (edgeElementsToAdd.length > 0) cy.add(edgeElementsToAdd);
    });

    const topLevelAddedIDs = topLevelNodeElementsToAdd.map((el: any) => String(el.data.id));
    const childAddedIDs = childNodeElementsToAdd.map((el: any) => String(el.data.id));
    const pendingPositionedIDs = new Set<string>();

    // Apply pending explicit positions first (e.g. palette drops)
    if (pendingNodePositionsRef && nodesToAdd.length > 0) {
      for (const id of nodesToAdd) {
        const pos = pendingNodePositionsRef.current.get(id);
        if (!pos) continue;
        const node = cy.getElementById(id) as cytoscape.NodeSingular;
        if (!node.length) continue;
        node.position(pos);
        pendingNodePositionsRef.current.delete(id);
        pendingPositionedIDs.add(id);
      }
    }

    // Place newly-added/reparented child nodes inside their parent containers.
    const needsChildPlacement = [...childAddedIDs, ...reparentedNodeIDs];
    for (const id of needsChildPlacement) {
      const child = cy.getElementById(id) as cytoscape.NodeSingular;
      if (!child.length || !child.isNode()) continue;
      const parent = child.parent()[0] as cytoscape.NodeSingular | undefined;
      if (!parent || !isContainerNode(parent)) continue;
      placeChildInParentSlot(child, parent);
    }

    // Position NEW top-level delta nodes around the action/selection anchor.
    if (!isFirstLoad && topLevelAddedIDs.length > 0) {
      const anchoredIDs = topLevelAddedIDs.filter(id => !pendingPositionedIDs.has(id));
      if (anchoredIDs.length > 0) {
        const pendingAnchor = pendingActionAnchorRef.current;
        const selectedCyNode = cy.nodes(':selected').first() as cytoscape.NodeSingular;
        const selectedByProp = selectedNodeId ? (cy.getElementById(selectedNodeId) as cytoscape.NodeSingular) : null;
        const lastAnchor = lastInteractionAnchorRef.current;
        const anchor =
          pendingAnchor
            ? (getNodePositionByID(cy, pendingAnchor.nodeId) ?? getViewportCenter(cy))
            : (selectedCyNode && selectedCyNode.length
              ? { ...selectedCyNode.position() }
              : (selectedByProp && selectedByProp.length
                ? { ...selectedByProp.position() }
                : (lastAnchor
                  ? (getNodePositionByID(cy, lastAnchor.nodeId) ?? getViewportCenter(cy))
                  : getViewportCenter(cy))));

        placeTopLevelNodesAroundAnchor(cy, anchoredIDs, anchor);
      }
    }
    if (hadPendingActionAnchor) {
      pendingActionAnchorRef.current = null;
    }
    enforceAllCompoundConstraints(cy);

    // Re-apply layer visibility only when new nodes arrived (existing nodes keep their class)
    if (hasAdded) syncLayerVisibility(cy, layerVisibilityRef.current);

    prevElementIdsRef.current = { nodes: newNodeIds, edges: newEdgeIds };

    // Auto-layout only on very first load; subsequent adds keep manual positions
    if (isFirstLoad) {
      const layout = cy.layout(dagreLayout);
      layout.on('layoutstop', () => {
        if (!cyRef.current) return;
        enforceAllCompoundConstraints(cyRef.current);
        cyRef.current.fit(undefined, 50);
      });
      layout.run();
    }
  }, [graphData, selectedNodeId]); // eslint-disable-line react-hooks/exhaustive-deps

  // --- Resize handle drag logic ---

  // Track active resize listeners so we can clean up if the component unmounts during a drag
  const resizeListenersRef = useRef<{ move: (e: MouseEvent) => void; up: () => void } | null>(null);

  // Remove any in-progress resize listeners (called from resize handlers and on cleanup)
  const cleanupResizeListeners = useCallback(() => {
    if (resizeListenersRef.current) {
      window.removeEventListener('mousemove', resizeListenersRef.current.move);
      window.removeEventListener('mouseup', resizeListenersRef.current.up);
      resizeListenersRef.current = null;
    }
  }, []);

  // Clean up window listeners if the component unmounts during an active drag
  useEffect(() => {
    return () => { cleanupResizeListeners(); };
  }, [cleanupResizeListeners]);

  const handleResizeMouseDown = useCallback((e: React.MouseEvent, corner: ResizeCorner) => {
    e.preventDefault();
    e.stopPropagation();

    const cy = cyRef.current;
    const id = selectedNodeIdRef.current;
    if (!cy || !id) return;
    const node = cy.getElementById(id);
    if (!node.length) return;

    const pos = node.position();
    resizeDragRef.current = {
      nodeId: id,
      corner,
      startMouseX: e.clientX,
      startMouseY: e.clientY,
      startModelW: node.width(),
      startModelH: node.height(),
      startModelCX: pos.x,
      startModelCY: pos.y,
    };

    const MIN = 40;

    const onMouseMove = (me: MouseEvent) => {
      const drag = resizeDragRef.current;
      if (!drag || !cyRef.current) return;

      const zoom = cyRef.current.zoom();
      const dxModel = (me.clientX - drag.startMouseX) / zoom;
      const dyModel = (me.clientY - drag.startMouseY) / zoom;

      let newW = drag.startModelW;
      let newH = drag.startModelH;
      let newCX = drag.startModelCX;
      let newCY = drag.startModelCY;

      switch (drag.corner) {
        case 'se':
          newW = Math.max(MIN, drag.startModelW + dxModel);
          newH = Math.max(MIN, drag.startModelH + dyModel);
          break;
        case 'sw':
          newW = Math.max(MIN, drag.startModelW - dxModel);
          newH = Math.max(MIN, drag.startModelH + dyModel);
          newCX = drag.startModelCX - (newW - drag.startModelW) / 2;
          break;
        case 'ne':
          newW = Math.max(MIN, drag.startModelW + dxModel);
          newH = Math.max(MIN, drag.startModelH - dyModel);
          newCY = drag.startModelCY - (newH - drag.startModelH) / 2;
          break;
        case 'nw':
          newW = Math.max(MIN, drag.startModelW - dxModel);
          newH = Math.max(MIN, drag.startModelH - dyModel);
          newCX = drag.startModelCX - (newW - drag.startModelW) / 2;
          newCY = drag.startModelCY - (newH - drag.startModelH) / 2;
          break;
      }

      const n = cyRef.current.getElementById(drag.nodeId) as cytoscape.NodeSingular;
      if (n.isParent() && isContainerNode(n)) {
        n.data('userMinW', newW);
        n.data('userMinH', newH);
        setNodeMinSize(n, newW, newH);
        n.position({ x: newCX, y: newCY });
        ensureParentContainsAllChildren(n);
      } else {
        n.style({ width: newW, height: newH });
        n.position({ x: newCX, y: newCY });
        const parent = n.parent()[0] as cytoscape.NodeSingular | undefined;
        if (parent) {
          ensureParentContainsChild(parent, n);
          clampChildInsideParent(n, parent);
        }
      }
      refreshResizeBB();
    };

    const onMouseUp = () => {
      resizeDragRef.current = null;
      cleanupResizeListeners();
    };

    resizeListenersRef.current = { move: onMouseMove, up: onMouseUp };
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
  }, [refreshResizeBB, cleanupResizeListeners]);

  // --- Layout controls ---

  const runLayout = useCallback((type: 'dagre' | 'cose' | 'grid') => {
    if (!cyRef.current) return;
    setLayoutType(type);
    const layouts = {
      dagre: dagreLayout,
      cose: { name: 'cose', animate: true, fit: true, padding: 30 },
      grid: { name: 'grid', animate: true, fit: true, padding: 30 },
    };
    const layout = cyRef.current.layout(layouts[type]);
    layout.on('layoutstop', () => {
      if (!cyRef.current) return;
      enforceAllCompoundConstraints(cyRef.current);
    });
    layout.run();
  }, []);

  const zoomIn = useCallback(() => {
    const cy = cyRef.current;
    if (cy) cy.zoom({ level: cy.zoom() * 1.2, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
  }, []);
  const zoomOut = useCallback(() => {
    const cy = cyRef.current;
    if (cy) cy.zoom({ level: cy.zoom() * 0.8, renderedPosition: { x: cy.width() / 2, y: cy.height() / 2 } });
  }, []);
  const fitToScreen = useCallback(() => { cyRef.current?.fit(undefined, 50); }, []);
  const resetView = useCallback(() => {
    if (cyRef.current) { cyRef.current.zoom(1); cyRef.current.center(); }
  }, []);

  return (
    <div className="graph-canvas-container">
      <div className="graph-controls">
        {/* Zoom + fit */}
        <div className="control-group">
          <button onClick={zoomIn} title="Zoom In">🔍+</button>
          <button onClick={zoomOut} title="Zoom Out">🔍-</button>
          <button onClick={fitToScreen} title="Fit to Screen">⛶</button>
          <button onClick={resetView} title="Reset View">↻</button>
        </div>

        {/* Layout algorithms */}
        <div className="control-group">
          <button onClick={() => runLayout('dagre')} className={layoutType === 'dagre' ? 'active' : ''}>Hierarchical</button>
          <button onClick={() => runLayout('cose')} className={layoutType === 'cose' ? 'active' : ''}>Force</button>
          <button onClick={() => runLayout('grid')} className={layoutType === 'grid' ? 'active' : ''}>Grid</button>
        </div>

        {/* Edge drawing mode */}
        <div className="control-group">
          <button
            onClick={handleToggleEdgeDraw}
            className={edgeDrawMode ? 'active' : ''}
            title={edgeDrawMode ? 'Exit edge draw mode (Esc)' : 'Draw edge: click source node, then target'}
          >
            ⤳ Edge
          </button>
        </div>

        {/* Layer visibility */}
        <div className="control-group" style={{ flexDirection: 'column', gap: 4 }}>
          <span className="control-label">Layers</span>
          <div style={{ display: 'flex', gap: 4 }}>
            {(Object.keys(LAYER_LABELS) as LayerGroup[]).map(group => (
              <button
                key={group}
                className={layerVisibility[group] ? 'active' : ''}
                title={`Toggle ${group} nodes`}
                onClick={() => setLayerVisibility(prev => ({ ...prev, [group]: !prev[group] }))}
              >
                {LAYER_LABELS[group]}
              </button>
            ))}
          </div>
        </div>

        {/* Legend */}
        <div className="control-group">
          <button onClick={() => setShowLegend(prev => !prev)} className={showLegend ? 'active' : ''} title="Node Type Legend">?</button>
        </div>
      </div>

      {/* Canvas area with resize overlay — flex:1 fills remaining height of the flex container */}
      <div style={{ position: 'relative', flex: 1, minHeight: 0 }}>
        <div ref={containerRef} className="graph-canvas" />

        {/* Resize overlay — positioned over selected node in canvas coords */}
        {resizeBB && (
          <div
            style={{
              position: 'absolute',
              left: resizeBB.x1,
              top: resizeBB.y1,
              width: resizeBB.w,
              height: resizeBB.h,
              pointerEvents: 'none',
              boxSizing: 'border-box',
              border: '1px dashed rgba(79, 193, 255, 0.6)',
              zIndex: 50,
            }}
          >
            {(['nw', 'ne', 'se', 'sw'] as ResizeCorner[]).map(corner => (
              <div
                key={corner}
                style={{
                  position: 'absolute',
                  width: HANDLE_SIZE,
                  height: HANDLE_SIZE,
                  background: '#ffffff',
                  border: '1.5px solid #4fc1ff',
                  borderRadius: 2,
                  cursor: CORNER_CURSOR[corner],
                  pointerEvents: 'all',
                  boxSizing: 'border-box',
                  ...CORNER_STYLE[corner],
                }}
                onMouseDown={(e) => handleResizeMouseDown(e, corner)}
              />
            ))}
          </div>
        )}

        {showLegend && <GraphLegend onClose={() => setShowLegend(false)} />}
      </div>
    </div>
  );
}
