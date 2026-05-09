// Three-panel layout with independent collapsible side panels

import { useState, useCallback } from 'react';
import { Panel, Group, Separator, usePanelRef, PanelSize } from 'react-resizable-panels';
import './ThreePanelLayout.css';

interface ThreePanelLayoutProps {
  leftPanel: React.ReactNode;
  centerPanel: React.ReactNode;
  rightPanel: React.ReactNode;
}

export function ThreePanelLayout({ leftPanel, centerPanel, rightPanel }: ThreePanelLayoutProps) {
  const [leftCollapsed, setLeftCollapsed] = useState(false);
  const [rightCollapsed, setRightCollapsed] = useState(false);
  
  const leftPanelRef = usePanelRef();
  const rightPanelRef = usePanelRef();

  const toggleLeftPanel = useCallback(() => {
    const panel = leftPanelRef.current;
    if (panel) {
      if (panel.isCollapsed()) {
        panel.expand();
        setLeftCollapsed(false);
      } else {
        panel.collapse();
        setLeftCollapsed(true);
      }
    }
  }, [leftPanelRef]);

  const toggleRightPanel = useCallback(() => {
    const panel = rightPanelRef.current;
    if (panel) {
      if (panel.isCollapsed()) {
        panel.expand();
        setRightCollapsed(false);
      } else {
        panel.collapse();
        setRightCollapsed(true);
      }
    }
  }, [rightPanelRef]);

  // Handle resize to track collapsed state
  const handleLeftResize = useCallback((size: PanelSize) => {
    setLeftCollapsed(size.asPercentage === 0);
  }, []);

  const handleRightResize = useCallback((size: PanelSize) => {
    setRightCollapsed(size.asPercentage === 0);
  }, []);

  return (
    <div className="three-panel-layout">
      <Group orientation="horizontal">
        {/* Left Panel - File Tree */}
        <Panel 
          id="left-panel"
          panelRef={leftPanelRef}
          defaultSize="20%"
          minSize="15%"
          maxSize="25%"
          collapsible={true}
          collapsedSize="0%"
          onResize={handleLeftResize}
        >
          <div className="panel-container left-panel">
            <div className="panel-header">
              <h3>File Tree</h3>
              <button 
                className="collapse-btn"
                onClick={toggleLeftPanel}
                title={leftCollapsed ? "Expand" : "Collapse"}
              >
                {leftCollapsed ? '▶' : '◀'}
              </button>
            </div>
            <div className="panel-content">
              {leftPanel}
            </div>
          </div>
        </Panel>

        <Separator className="resize-handle" />

        {/* Center Panel - Graph Canvas */}
        <Panel id="center-panel" minSize="30%">
          <div className="panel-container center-panel">
            {/* Expand button for collapsed left panel */}
            {leftCollapsed && (
              <button 
                className="expand-edge-btn left"
                onClick={toggleLeftPanel}
                title="Show File Tree"
              >
                <span className="expand-icon">▶</span>
                <span className="expand-label">Files</span>
              </button>
            )}
            
            {centerPanel}
            
            {/* Expand button for collapsed right panel */}
            {rightCollapsed && (
              <button 
                className="expand-edge-btn right"
                onClick={toggleRightPanel}
                title="Show Info Panel"
              >
                <span className="expand-icon">◀</span>
                <span className="expand-label">Info</span>
              </button>
            )}
          </div>
        </Panel>

        <Separator className="resize-handle" />

        {/* Right Panel - Info Panel */}
        <Panel 
          id="right-panel"
          panelRef={rightPanelRef}
          defaultSize="20%"
          minSize="15%"
          maxSize="25%"
          collapsible={true}
          collapsedSize="0%"
          onResize={handleRightResize}
        >
          <div className="panel-container right-panel">
            <div className="panel-header">
              <h3>Info Panel</h3>
              <button 
                className="collapse-btn"
                onClick={toggleRightPanel}
                title={rightCollapsed ? "Expand" : "Collapse"}
              >
                {rightCollapsed ? '◀' : '▶'}
              </button>
            </div>
            <div className="panel-content">
              {rightPanel}
            </div>
          </div>
        </Panel>
      </Group>
    </div>
  );
}
