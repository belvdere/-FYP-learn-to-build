// Cytoscape layout configurations

export const dagreLayout = {
  name: 'dagre',
  rankDir: 'TB', // Top to bottom (or 'LR' for left to right)
  align: 'UL',
  nodeSep: 80,
  rankSep: 120,
  padding: 30,
  animate: true,
  animationDuration: 500,
  fit: true,
};

export const concentricLayout = {
  name: 'concentric',
  fit: true,
  padding: 30,
  startAngle: 3.14159 / 2,
  sweep: undefined,
  clockwise: true,
  equidistant: false,
  minNodeSpacing: 100,
  animate: true,
  animationDuration: 500,
};

export const gridLayout = {
  name: 'grid',
  fit: true,
  padding: 30,
  avoidOverlap: true,
  avoidOverlapPadding: 10,
  animate: true,
  animationDuration: 500,
};

export const coseLayout = {
  name: 'cose',
  animate: true,
  animationDuration: 500,
  fit: true,
  padding: 30,
  nodeRepulsion: 400000,
  idealEdgeLength: 100,
  edgeElasticity: 100,
  nestingFactor: 5,
  gravity: 80,
  numIter: 1000,
  initialTemp: 200,
  coolingFactor: 0.95,
  minTemp: 1.0,
};

export const layoutOptions = {
  dagre: dagreLayout,
  concentric: concentricLayout,
  grid: gridLayout,
  cose: coseLayout,
};

