// Shape palette toolbar for placing virtual nodes on the canvas

export type PlacementTool = 'virtual-directory' | 'virtual-class' | 'virtual';

interface ShapePaletteProps {
  activeTool: PlacementTool | null;
  onToolSelect: (tool: PlacementTool | null) => void;
}

const tools: { type: PlacementTool; label: string; icon: string; color: string }[] = [
  { type: 'virtual-directory', label: 'Virtual Directory', icon: '⬜', color: '#ce9178' },
  { type: 'virtual-class', label: 'Virtual Class', icon: '⬛', color: '#c586c0' },
  { type: 'virtual', label: 'Virtual Method', icon: '◇', color: '#c586c0' },
];

export function ShapePalette({ activeTool, onToolSelect }: ShapePaletteProps) {
  const handleClick = (type: PlacementTool) => {
    onToolSelect(activeTool === type ? null : type);
  };

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      gap: 6,
      padding: '6px 10px',
      background: '#1e1e1e',
      borderBottom: '1px solid #3e3e3e',
      flexShrink: 0,
    }}>
      <span style={{ fontSize: 11, color: '#888', marginRight: 4 }}>Place:</span>
      {tools.map(tool => (
        <button
          key={tool.type}
          onClick={() => handleClick(tool.type)}
          title={tool.label}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 4,
            padding: '3px 10px',
            background: activeTool === tool.type ? '#37373d' : 'transparent',
            border: `1px solid ${activeTool === tool.type ? tool.color : '#3e3e3e'}`,
            borderRadius: 3,
            color: tool.color,
            cursor: 'pointer',
            fontSize: 12,
            outline: 'none',
          }}
        >
          <span style={{ fontSize: 14 }}>{tool.icon}</span>
          <span>{tool.label}</span>
        </button>
      ))}
      {activeTool && (
        <span style={{ marginLeft: 8, fontSize: 11, color: '#888', fontStyle: 'italic' }}>
          Click canvas to place · Esc to cancel
        </span>
      )}
    </div>
  );
}
