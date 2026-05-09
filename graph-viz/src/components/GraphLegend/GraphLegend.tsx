// Floating legend panel describing node types on the canvas

interface GraphLegendProps {
  onClose: () => void;
}

interface LegendEntry {
  swatch: string;      // CSS background color
  border: string;      // CSS border color
  borderStyle: string; // solid | dashed
  shape: string;       // box | circle | hexagon | diamond
  type: string;
  description: string;
}

const LEGEND_ENTRIES: LegendEntry[] = [
  { swatch: '#1a3a2a', border: '#4a9a6a', borderStyle: 'solid', shape: 'box', type: 'Real Directory', description: 'Filesystem directory' },
  { swatch: '#2a3a4a', border: '#7aafd4', borderStyle: 'solid', shape: 'box', type: 'Real File', description: 'Source file / class' },
  { swatch: '#2a3a4a', border: '#7aafd4', borderStyle: 'solid', shape: 'circle', type: 'Real Method', description: 'Method or constructor' },
  { swatch: '#2a2010', border: '#ce9178', borderStyle: 'dashed', shape: 'box', type: 'Virtual Directory', description: 'User-defined container for virtual classes' },
  { swatch: '#2a1e2a', border: '#c586c0', borderStyle: 'dashed', shape: 'box', type: 'Virtual Class', description: 'User-defined class to be generated' },
  { swatch: '#3b2a44', border: '#c586c0', borderStyle: 'solid', shape: 'diamond', type: 'Virtual Method', description: 'User-defined method / concept' },
];

function Swatch({ entry }: { entry: LegendEntry }) {
  const isHexagon = entry.shape === 'hexagon';
  const isCircle = entry.shape === 'circle';
  const isDiamond = entry.shape === 'diamond';

  const baseStyle: React.CSSProperties = {
    width: 18,
    height: 18,
    background: entry.swatch,
    border: `2px ${entry.borderStyle} ${entry.border}`,
    flexShrink: 0,
  };

  if (isCircle) {
    return <div style={{ ...baseStyle, borderRadius: '50%' }} />;
  }
  if (isDiamond) {
    return (
      <div style={{ width: 18, height: 18, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <div style={{ ...baseStyle, width: 13, height: 13, transform: 'rotate(45deg)', borderRadius: 0 }} />
      </div>
    );
  }
  if (isHexagon) {
    return (
      <div style={{ width: 18, height: 18, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <div style={{
          width: 16,
          height: 9,
          background: entry.swatch,
          border: `2px solid ${entry.border}`,
          position: 'relative',
          clipPath: 'polygon(25% 0%, 75% 0%, 100% 50%, 75% 100%, 25% 100%, 0% 50%)',
        }} />
      </div>
    );
  }
  return <div style={{ ...baseStyle, borderRadius: 2 }} />;
}

export function GraphLegend({ onClose }: GraphLegendProps) {
  return (
    <div style={{
      position: 'absolute',
      top: 48,
      right: 8,
      zIndex: 1100,
      background: '#252526',
      border: '1px solid #3e3e3e',
      borderRadius: 4,
      padding: '12px 14px',
      minWidth: 280,
      boxShadow: '0 4px 12px rgba(0,0,0,0.6)',
      color: '#cccccc',
      fontSize: 12,
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
        <span style={{ fontWeight: 'bold', fontSize: 13 }}>Node Types</span>
        <button
          onClick={onClose}
          style={{ background: 'none', border: 'none', color: '#cccccc', cursor: 'pointer', fontSize: 14, padding: '0 4px', lineHeight: 1 }}
        >✕</button>
      </div>
      <table style={{ borderCollapse: 'collapse', width: '100%' }}>
        <tbody>
          {LEGEND_ENTRIES.map(entry => (
            <tr key={entry.type} style={{ verticalAlign: 'middle' }}>
              <td style={{ padding: '4px 8px 4px 0' }}>
                <Swatch entry={entry} />
              </td>
              <td style={{ padding: '4px 8px 4px 0', fontWeight: 500, whiteSpace: 'nowrap' }}>{entry.type}</td>
              <td style={{ padding: '4px 0', color: '#888' }}>{entry.description}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
