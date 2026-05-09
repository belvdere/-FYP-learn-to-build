// Cytoscape.js styling — supports conditional compound hierarchy rendering.

export const cytoscapeStyles: any = [
  // Real Directory — dark green tint with green border
  {
    selector: 'node[type="directory"]',
    style: {
      shape: 'roundrectangle',
      'background-color': '#1a3a2a',
      'background-opacity': 0.45,
      'border-color': '#4a9a6a',
      'border-width': 2,
      color: '#7ac89a',
      'font-size': 11,
      'font-weight': 'bold',
      label: 'data(label)',
      'text-valign': 'top',
      'text-halign': 'center',
      'text-margin-x': 0,
      'text-margin-y': 16,
      'text-wrap': 'none',
      width: 120,
      height: 50,
      'z-index': 10,
    },
  },

  // Real File / Class — steel-blue tint with blue border
  {
    selector: 'node[type="file"]',
    style: {
      shape: 'roundrectangle',
      'background-color': '#2a3a4a',
      'background-opacity': 0.35,
      'border-color': '#7aafd4',
      'border-width': 2,
      color: '#a8c8e8',
      'font-size': 11,
      'font-weight': 'bold',
      label: 'data(label)',
      'text-valign': 'top',
      'text-halign': 'center',
      'text-margin-x': 0,
      'text-margin-y': 16,
      'text-wrap': 'none',
      width: 120,
      height: 50,
      'z-index': 20,
    },
  },

  // Compound parent behavior (applies when a node has visible children)
  {
    selector: 'node:parent',
    style: {
      'text-valign': 'top',
      'text-halign': 'center',
      'text-margin-y': 12,
      'padding': 18,
      'compound-sizing-wrt-labels': 'exclude',
      'min-width': 120,
      'min-height': 70,
    },
  },

  // Real Method — translucent blue with bold border
  {
    selector: 'node[type="method"]',
    style: {
      shape: 'ellipse',
      'background-color': '#2a3a4a',
      'background-opacity': 0.35,
      'border-color': '#7aafd4',
      'border-width': 3,
      color: '#7aafd4',
      'font-size': 10,
      label: 'data(label)',
      'text-valign': 'center',
      'text-halign': 'center',
      'text-wrap': 'wrap',
      'text-max-width': '90px',
      width: 100,
      height: 100,
      'z-index': 30,
    },
  },

  // Real Field — teal filled rectangle
  {
    selector: 'node[type="field"]',
    style: {
      shape: 'rectangle',
      'background-color': '#1a3a3a',
      'background-opacity': 0.8,
      'border-color': '#4ec9b0',
      'border-width': 2,
      color: '#4ec9b0',
      'font-size': 10,
      label: 'data(label)',
      'text-valign': 'center',
      'text-halign': 'center',
      'text-wrap': 'wrap',
      'text-max-width': '90px',
      width: 100,
      height: 40,
      'z-index': 30,
    },
  },

  // Virtual Directory — hollow orange dashed square
  {
    selector: 'node[type="virtual-directory"]',
    style: {
      shape: 'roundrectangle',
      'background-opacity': 0,
      'border-color': '#ce9178',
      'border-style': 'dashed',
      'border-width': 2,
      color: '#ce9178',
      'font-size': 11,
      'font-weight': 'bold',
      label: 'data(label)',
      'text-valign': 'top',
      'text-halign': 'center',
      'text-margin-x': 0,
      'text-margin-y': 16,
      'text-wrap': 'none',
      width: 140,
      height: 55,
      'z-index': 10,
    },
  },

  // Virtual Class — hollow purple dashed square
  {
    selector: 'node[type="virtual-class"]',
    style: {
      shape: 'roundrectangle',
      'background-opacity': 0,
      'border-color': '#c586c0',
      'border-style': 'dashed',
      'border-width': 2,
      color: '#c586c0',
      'font-size': 11,
      'font-weight': 'bold',
      label: 'data(label)',
      'text-valign': 'top',
      'text-halign': 'center',
      'text-margin-x': 0,
      'text-margin-y': 16,
      'text-wrap': 'none',
      width: 140,
      height: 55,
      'z-index': 20,
    },
  },

  // Virtual Method — translucent purple with bold border
  {
    selector: 'node[type="virtual"]',
    style: {
      shape: 'diamond',
      'background-color': '#3b2a44',
      'background-opacity': 0.35,
      'border-color': '#c586c0',
      'border-width': 3,
      color: '#c586c0',
      'font-size': 10,
      label: 'data(label)',
      'text-valign': 'center',
      'text-halign': 'center',
      'text-wrap': 'wrap',
      'text-max-width': '90px',
      width: 110,
      height: 110,
      'z-index': 30,
    },
  },

  // Stale snapshot nodes (referenced in snapshot but no longer in code index)
  {
    selector: 'node[?isStaleSnapshot]',
    style: {
      'border-color': '#cc4444',
      'border-style': 'dashed',
      'border-width': 2,
      opacity: 0.5,
    },
  },

  // Stale snapshot edges
  {
    selector: 'edge[?isStaleSnapshot]',
    style: {
      'line-color': '#cc4444',
      'target-arrow-color': '#cc4444',
      'line-style': 'dashed',
      opacity: 0.5,
    },
  },

  // Layer hidden (toggled off via layer panel)
  {
    selector: 'node.layer-hidden',
    style: { display: 'none' },
  },

  // Selected node
  {
    selector: ':selected',
    style: {
      'border-color': '#f48771',
      'border-width': 4,
      'overlay-color': '#f48771',
      'overlay-padding': 4,
      'overlay-opacity': 0.2,
    },
  },

  // Hover / active
  {
    selector: 'node:active',
    style: {
      'overlay-color': '#4fc1ff',
      'overlay-padding': 8,
      'overlay-opacity': 0.25,
    },
  },

  // Edges — same z-index as method nodes so they render above directory/file layers
  {
    selector: 'edge',
    style: {
      width: 2,
      'line-color': '#6a6a6a',
      'target-arrow-color': '#6a6a6a',
      'target-arrow-shape': 'triangle',
      'curve-style': 'bezier',
      'arrow-scale': 1.2,
      'z-index': 30,
    },
  },

  // Virtual / manual edges
  {
    selector: 'edge[?isVirtual]',
    style: {
      'line-color': '#8b5a2b',
      'target-arrow-color': '#8b5a2b',
    },
  },

  // Edges adjacent to a selected node
  {
    selector: 'edge.edge-highlighted',
    style: {
      width: 3,
      'line-color': '#f48771',
      'target-arrow-color': '#f48771',
    },
  },

  // Selected edge — dark yellow
  {
    selector: 'edge:selected',
    style: {
      width: 4,
      'line-color': '#d4a017',
      'target-arrow-color': '#d4a017',
      'overlay-color': '#d4a017',
      'overlay-padding': 3,
      'overlay-opacity': 0.3,
    },
  },

  // Edge handle styles (cytoscape-edgehandles)
  {
    selector: '.eh-handle',
    style: {
      'background-color': '#4fc1ff',
      width: 12,
      height: 12,
      shape: 'ellipse',
      'overlay-opacity': 0,
    },
  },
  {
    selector: '.eh-hover',
    style: {
      'background-color': '#4fc1ff',
      'background-opacity': 0.3,
    },
  },
  {
    selector: '.eh-ghost-edge',
    style: {
      'line-color': '#4fc1ff',
      'target-arrow-color': '#4fc1ff',
      'line-style': 'dashed',
    },
  },
  {
    selector: 'edge.eh-preview, .eh-ghost-edge.eh-preview-active',
    style: {
      opacity: 0,
    },
  },
];
