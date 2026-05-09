// Info panel component for displaying node/edge details

import { useState, useEffect, useRef } from 'react';
import { GraphNode, GraphEdge, NodeAnnotation, EdgeAnnotation } from '../../types/graph';
import { CreateVirtualNodeRequest, CreateEdgeRequest, SnapshotNode, SnapshotEdge } from '../../types/api';
import { graphApi } from '../../services/graphApi';
import { postMessage } from '../../services/webviewTransport';
import './InfoPanel.css';

interface InfoPanelProps {
  selectedNode?: GraphNode | null;
  selectedEdge?: GraphEdge | null;
  existingNodes?: GraphNode[];
  activeNodes?: GraphNode[];  // Nodes currently displayed in graph
  activeEdges?: GraphEdge[];  // Edges currently displayed in graph
  onAnnotationUpdate?: () => void;
  onNodeCreated?: (node: GraphNode) => void;
  onEdgeCreated?: (edge: GraphEdge) => void;
  onNodeDeleted?: (nodeId: string) => void;
  onEdgeDeleted?: (edgeId: string) => void;
}

export function InfoPanel({
  selectedNode,
  selectedEdge,
  existingNodes = [],
  activeNodes = [],
  activeEdges = [],
  onAnnotationUpdate,
  onNodeCreated,
  onEdgeCreated,
  onNodeDeleted,
  onEdgeDeleted,
}: InfoPanelProps) {
  if (!selectedNode && !selectedEdge) {
    return (
      <div className="info-panel">
        <CreateSection
          existingNodes={existingNodes}
          activeNodes={activeNodes}
          activeEdges={activeEdges}
          onNodeCreated={onNodeCreated}
          onEdgeCreated={onEdgeCreated}
        />
      </div>
    );
  }

  if (selectedNode) {
    return (
      <NodeInfo 
        node={selectedNode} 
        onUpdate={onAnnotationUpdate} 
        onDelete={onNodeDeleted} 
      />
    );
  }

  if (selectedEdge) {
    // An edge is user-created if the backend flagged it as manual OR it connects to a virtual node
    const isUserCreated = selectedEdge.isManual || existingNodes.some(n => 
      n.isVirtual && (n.id === selectedEdge.source || n.id === selectedEdge.target)
    );
    return (
      <EdgeInfo 
        edge={selectedEdge} 
        isVirtual={isUserCreated}
        onUpdate={onAnnotationUpdate} 
        onDelete={onEdgeDeleted}
      />
    );
  }

  return null;
}

// Create section for adding virtual nodes and edges
interface CreateSectionProps {
  existingNodes: GraphNode[];
  activeNodes: GraphNode[];
  activeEdges: GraphEdge[];
  onNodeCreated?: (node: GraphNode) => void;
  onEdgeCreated?: (edge: GraphEdge) => void;
}

function CreateSection({ existingNodes, activeNodes, activeEdges, onNodeCreated, onEdgeCreated }: CreateSectionProps) {
  const [activeTab, setActiveTab] = useState<'node' | 'edge' | 'snapshot'>('node');
  const [saving, setSaving] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ text: string; ok: boolean } | null>(null);
  const statusTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => { if (statusTimerRef.current) clearTimeout(statusTimerRef.current); };
  }, []);

  const showStatus = (text: string, ok: boolean) => {
    if (statusTimerRef.current) clearTimeout(statusTimerRef.current);
    setStatusMsg({ text, ok });
    statusTimerRef.current = setTimeout(() => { statusTimerRef.current = null; setStatusMsg(null); }, 3000);
  };
  
  // Snapshot state
  const [snapshotName, setSnapshotName] = useState('');
  const [createdSnapshotId, setCreatedSnapshotId] = useState<string | null>(null);
  const [snapshotCopied, setSnapshotCopied] = useState(false);
  
  // Node form state
  const [nodeLabel, setNodeLabel] = useState('');
  const [nodeType, setNodeType] = useState('method');
  const [nodeClassName, setNodeClassName] = useState('');
  const [nodeFilePath, setNodeFilePath] = useState('');
  const [nodeDescription, setNodeDescription] = useState('');
  const [nodeAiRemarks, setNodeAiRemarks] = useState('');
  
  // Edge form state
  const [edgeSource, setEdgeSource] = useState('');
  const [edgeTarget, setEdgeTarget] = useState('');
  const [sourceSearch, setSourceSearch] = useState('');
  const [targetSearch, setTargetSearch] = useState('');

  const nodeTypes = ['class', 'method', 'module'];
  
  // Build the full label including class name if provided
  const getFullLabel = () => {
    if (nodeType === 'class') return nodeClassName.trim();
    if (nodeClassName && nodeLabel) return `${nodeClassName}.${nodeLabel}`;
    return nodeLabel;
  };

  const filteredSourceNodes = existingNodes.filter(n => 
    n.label.toLowerCase().includes(sourceSearch.toLowerCase()) ||
    n.id.toLowerCase().includes(sourceSearch.toLowerCase())
  );

  const filteredTargetNodes = existingNodes.filter(n => 
    n.label.toLowerCase().includes(targetSearch.toLowerCase()) ||
    n.id.toLowerCase().includes(targetSearch.toLowerCase())
  );

  const handleCreateNode = async () => {
    const labelToUse = nodeType === 'class' ? nodeClassName.trim() : nodeLabel.trim();
    if (!labelToUse) {
      showStatus(nodeType === 'class' ? 'Please enter a class name' : 'Please enter a node label', false);
      return;
    }

    setSaving(true);
    try {
      const fullLabel = getFullLabel();
      const request: CreateVirtualNodeRequest = {
        label: fullLabel.trim(),
        type: nodeType === 'module' ? 'directory' : nodeType,
        virtualDirectory: nodeFilePath.trim() || undefined,
      };
      
      const createdNode = await graphApi.createVirtualNode(request);
      
      // Update annotation if provided
      if (nodeDescription || nodeAiRemarks) {
        await graphApi.updateNodeAnnotation(createdNode.id, {
          description: nodeDescription || undefined,
          aiRemarks: nodeAiRemarks || undefined,
        });
        createdNode.annotation = {
          description: nodeDescription,
          aiRemarks: nodeAiRemarks,
        };
      }
      
      if (onNodeCreated) {
        onNodeCreated(createdNode);
      }

      // Reset form
      setNodeLabel('');
      setNodeClassName('');
      setNodeFilePath('');
      setNodeType('method');
      setNodeDescription('');
      setNodeAiRemarks('');

      showStatus(`Node "${createdNode.label}" created`, true);
    } catch (error) {
      console.error('Failed to create node:', error);
      showStatus('Failed to create node', false);
    } finally {
      setSaving(false);
    }
  };

  const handleCreateEdge = async () => {
    if (!edgeSource || !edgeTarget) {
      showStatus('Please select both source and target nodes', false);
      return;
    }

    if (edgeSource === edgeTarget) {
      showStatus('Source and target must be different nodes', false);
      return;
    }

    setSaving(true);
    try {
      const request: CreateEdgeRequest = {
        sourceNodeId: edgeSource,
        targetNodeId: edgeTarget,
      };
      
      const createdEdge = await graphApi.createEdge(request);
      
      if (onEdgeCreated) {
        onEdgeCreated(createdEdge);
      }

      // Reset form
      setEdgeSource('');
      setEdgeTarget('');
      setSourceSearch('');
      setTargetSearch('');

      showStatus('Edge created', true);
    } catch (error) {
      console.error('Failed to create edge:', error);
      const detail = error instanceof Error ? error.message : String(error);
      showStatus(`Failed to create edge: ${detail}`, false);
    } finally {
      setSaving(false);
    }
  };

  const handleCreateSnapshot = async () => {
    if (activeNodes.length === 0) {
      showStatus('Add nodes to the graph first', false);
      return;
    }

    setSaving(true);
    setCreatedSnapshotId(null);
    
    try {
      const inferSnapshotContextType = (node: GraphNode): string => {
        if (node.type === 'directory') return 'directory';
        const fileName = node.filePath.split(/[\\/]/).pop() || '';
        if (
          !node.isVirtual &&
          (node.type === 'class' || node.type === 'interface') &&
          node.line === 0 &&
          !!node.filePath &&
          node.label === fileName
        ) {
          return 'file';
        }
        return node.type;
      };

      // Separate virtual and context nodes
      const virtualNodes: SnapshotNode[] = activeNodes
        .filter(n => n.isVirtual)
        .map(n => ({
          id: n.id,
          label: n.label,
          type: n.type,
          filePath: n.filePath,
          line: n.line,
          isVirtual: true,
          parentId: n.parentId,
          description: n.annotation?.description,
          aiRemarks: n.annotation?.aiRemarks,
        }));

      const contextNodes: SnapshotNode[] = activeNodes
        .filter(n => !n.isVirtual)
        .map(n => ({
          id: n.id,
          label: n.label,
          type: inferSnapshotContextType(n),
          filePath: n.filePath,
          line: n.line,
          isVirtual: false,
          parentId: n.parentId,
          description: n.annotation?.description,
          aiRemarks: n.annotation?.aiRemarks,
        }));

      const edges: SnapshotEdge[] = activeEdges.map(e => ({
        id: e.id,
        sourceId: e.source,
        targetId: e.target,
        remarks: e.annotation?.remarks,
      }));

      const snapshot = await graphApi.createSnapshot({
        name: snapshotName.trim() || undefined,
        virtualNodes,
        contextNodes,
        edges,
      });

      setCreatedSnapshotId(snapshot.id);
      setSnapshotName('');
    } catch (error) {
      console.error('Failed to create snapshot:', error);
      showStatus('Failed to create snapshot', false);
    } finally {
      setSaving(false);
    }
  };

  const handleCopySnapshotId = async () => {
    if (!createdSnapshotId) return;
    
    try {
      await navigator.clipboard.writeText(createdSnapshotId);
      setSnapshotCopied(true);
      setTimeout(() => setSnapshotCopied(false), 2000);
    } catch (error) {
      // Fallback
      const textArea = document.createElement('textarea');
      textArea.value = createdSnapshotId;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
      setSnapshotCopied(true);
      setTimeout(() => setSnapshotCopied(false), 2000);
    }
  };

  return (
    <>
      <div className="info-panel-header">
        <h3>Create New</h3>
      </div>

      {statusMsg && (
        <div style={{
          margin: '0 12px 8px',
          padding: '6px 10px',
          borderRadius: 4,
          fontSize: 12,
          background: statusMsg.ok ? 'rgba(78,201,176,0.15)' : 'rgba(244,135,113,0.15)',
          border: `1px solid ${statusMsg.ok ? '#4ec9b0' : '#f48771'}`,
          color: statusMsg.ok ? '#4ec9b0' : '#f48771',
        }}>
          {statusMsg.text}
        </div>
      )}

      <div className="create-tabs">
        <button 
          className={`tab-btn ${activeTab === 'node' ? 'active' : ''}`}
          onClick={() => setActiveTab('node')}
        >
          + Node
        </button>
        <button 
          className={`tab-btn ${activeTab === 'edge' ? 'active' : ''}`}
          onClick={() => setActiveTab('edge')}
        >
          + Edge
        </button>
        <button 
          className={`tab-btn ${activeTab === 'snapshot' ? 'active' : ''}`}
          onClick={() => setActiveTab('snapshot')}
        >
          📸 Snapshot
        </button>
      </div>

      {activeTab === 'node' && (
        <div className="create-form">
          <div className="info-section">
            <label>{nodeType === 'class' ? 'Class Name: *' : 'Class Name:'}</label>
            <input
              type="text"
              value={nodeClassName}
              onChange={(e) => setNodeClassName(e.target.value)}
              placeholder="e.g., UserService"
              className="form-input"
            />
          </div>

          {nodeType !== 'class' && (
            <div className="info-section">
              <label>Method/Field Name: *</label>
              <input
                type="text"
                value={nodeLabel}
                onChange={(e) => setNodeLabel(e.target.value)}
                placeholder="e.g., getUserById"
                className="form-input"
              />
              {nodeClassName && nodeLabel && (
                <div className="label-preview">
                  Full label: <code>{getFullLabel()}</code>
                </div>
              )}
            </div>
          )}
          
          <div className="info-section">
            <label>Type:</label>
            <select 
              value={nodeType} 
              onChange={(e) => setNodeType(e.target.value)}
              className="form-select"
            >
              {nodeTypes.map(type => (
                <option key={type} value={type}>{type}</option>
              ))}
            </select>
          </div>
          

          <div className="info-section">
            <label>File Path:</label>
            <input
              type="text"
              value={nodeFilePath}
              onChange={(e) => setNodeFilePath(e.target.value)}
              placeholder="e.g., src/services/user"
              className="form-input"
            />
            <div className="field-hint">Directory where this virtual node belongs</div>
          </div>
          
          <div className="info-section">
            <label>Description:</label>
            <textarea
              value={nodeDescription}
              onChange={(e) => setNodeDescription(e.target.value)}
              placeholder="What does this represent?"
              rows={3}
            />
          </div>
          
          <div className="info-section">
            <label>AI Remarks:</label>
            <textarea
              value={nodeAiRemarks}
              onChange={(e) => setNodeAiRemarks(e.target.value)}
              placeholder="Instructions for AI code generation..."
              rows={3}
            />
          </div>
          
          <div className="info-actions">
            <button
              onClick={handleCreateNode}
              disabled={saving || (nodeType === 'class' ? !nodeClassName.trim() : !nodeLabel.trim())}
              className="save-btn"
            >
              {saving ? 'Creating...' : 'Create Node'}
            </button>
          </div>
        </div>
      )}

      {activeTab === 'edge' && (
        <div className="create-form">
          <div className="info-section">
            <label>Source Node: *</label>
            <input
              type="text"
              value={sourceSearch}
              onChange={(e) => setSourceSearch(e.target.value)}
              placeholder="Search nodes..."
              className="form-input"
            />
            {sourceSearch && filteredSourceNodes.length > 0 && (
              <div className="node-dropdown">
                {filteredSourceNodes.slice(0, 8).map(node => (
                  <div 
                    key={node.id}
                    className={`node-option ${edgeSource === node.id ? 'selected' : ''}`}
                    onClick={() => {
                      setEdgeSource(node.id);
                      setSourceSearch(node.label);
                    }}
                  >
                    <span className="node-label">{node.label}</span>
                    <span className="node-type">{node.type}</span>
                  </div>
                ))}
              </div>
            )}
            {edgeSource && (
              <div className="selected-node">
                Selected: {existingNodes.find(n => n.id === edgeSource)?.label || edgeSource}
              </div>
            )}
          </div>
          
          <div className="info-section">
            <label>Target Node: *</label>
            <input
              type="text"
              value={targetSearch}
              onChange={(e) => setTargetSearch(e.target.value)}
              placeholder="Search nodes..."
              className="form-input"
            />
            {targetSearch && filteredTargetNodes.length > 0 && (
              <div className="node-dropdown">
                {filteredTargetNodes.slice(0, 8).map(node => (
                  <div 
                    key={node.id}
                    className={`node-option ${edgeTarget === node.id ? 'selected' : ''}`}
                    onClick={() => {
                      setEdgeTarget(node.id);
                      setTargetSearch(node.label);
                    }}
                  >
                    <span className="node-label">{node.label}</span>
                    <span className="node-type">{node.type}</span>
                  </div>
                ))}
              </div>
            )}
            {edgeTarget && (
              <div className="selected-node">
                Selected: {existingNodes.find(n => n.id === edgeTarget)?.label || edgeTarget}
              </div>
            )}
          </div>
          
          <div className="info-actions">
            <button 
              onClick={handleCreateEdge} 
              disabled={saving || !edgeSource || !edgeTarget} 
              className="save-btn"
            >
              {saving ? 'Creating...' : 'Create Edge'}
            </button>
          </div>
          
          {existingNodes.length === 0 && (
            <div className="create-hint">
              <p>Load nodes first using "Load All" button, then you can create edges between them.</p>
            </div>
          )}
        </div>
      )}

      {activeTab === 'snapshot' && (
        <div className="create-form">
          <div className="info-section">
            <label>Snapshot Name (optional):</label>
            <input
              type="text"
              value={snapshotName}
              onChange={(e) => setSnapshotName(e.target.value)}
              placeholder="e.g., Auth flow v1"
              className="form-input"
            />
          </div>

          <div className="info-section">
            <label>Current Graph State:</label>
            <div className="snapshot-stats">
              <span>🎯 {activeNodes.filter(n => n.isVirtual).length} virtual nodes</span>
              <span>📄 {activeNodes.filter(n => !n.isVirtual).length} context nodes</span>
              <span>🔗 {activeEdges.length} edges</span>
            </div>
          </div>

          <div className="info-actions">
            <button 
              onClick={handleCreateSnapshot} 
              disabled={saving || activeNodes.length === 0} 
              className="save-btn"
            >
              {saving ? 'Capturing...' : '📸 Capture Snapshot'}
            </button>
          </div>

          {createdSnapshotId && (
            <div className="snapshot-created">
              <div className="snapshot-created-header">Snapshot Created!</div>
              <div className="snapshot-id-display">
                <code>{createdSnapshotId}</code>
                <button 
                  onClick={handleCopySnapshotId}
                  className={`copy-btn ${snapshotCopied ? 'copied' : ''}`}
                >
                  {snapshotCopied ? '✓ Copied!' : '📋 Copy'}
                </button>
              </div>
              <div className="snapshot-usage-hint">
                Use this ID in Copilot Chat:<br/>
                <code>"Generate code using snapshot {createdSnapshotId}"</code>
              </div>
            </div>
          )}

          {activeNodes.length === 0 && (
            <div className="create-hint">
              <p>Add nodes to the graph first to create a snapshot.</p>
            </div>
          )}
        </div>
      )}
      
      <div className="info-panel-help" style={{ marginTop: 'auto', padding: '16px' }}>
        <h4>Quick Tips:</h4>
        <ul>
          <li><strong>Click</strong> a node in the file tree to add it to the graph</li>
          <li><strong>Double-click</strong> a graph node to expand its neighbors</li>
          <li><strong>Right-click</strong> a graph node to render its parent</li>
          <li><strong>Delete-key</strong> a selected graph node to remove from the graph</li>
        </ul>
      </div>
    </>
  );
}

interface NodeInfoProps {
  node: GraphNode;
  onUpdate?: () => void;
  onDelete?: (nodeId: string) => void;
}

function NodeInfo({ node, onUpdate, onDelete }: NodeInfoProps) {
  const [editing, setEditing] = useState(false);
  const [labelInput, setLabelInput] = useState(node.label);
  const [virtualDirInput, setVirtualDirInput] = useState(node.filePath || '');
  const [annotation, setAnnotation] = useState<NodeAnnotation>(node.annotation || {});
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Reset all editable state when the selected node identity changes.
  // Depends only on node.id — annotation is an object reference that changes on every
  // parent render, and including it would reset the form mid-edit.
  useEffect(() => {
    setLabelInput(node.label);
    setVirtualDirInput(node.filePath || '');
    setAnnotation(node.annotation || {});
    setEditing(false);
    setShowDeleteConfirm(false);
  }, [node.id]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSave = async () => {
    setSaving(true);
    try {
      if (node.isVirtual) {
        const updates: Record<string, string | undefined> = {};
        if (labelInput.trim() && labelInput.trim() !== node.label) {
          updates.label = labelInput.trim();
        }
        const newDir = virtualDirInput.trim() || undefined;
        if (newDir !== (node.filePath || undefined)) {
          updates.virtualDirectory = newDir;
        }
        if (Object.keys(updates).length > 0) {
          await graphApi.updateVirtualNode(node.id, updates);
        }
      }
      await graphApi.updateNodeAnnotation(node.id, annotation);
      setEditing(false);
      if (onUpdate) onUpdate();
    } catch (error) {
      console.error('Failed to save annotation:', error);
      postMessage({ type: 'showError', text: 'Failed to save annotation' });
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setLabelInput(node.label);
    setAnnotation(node.annotation || {});
    setEditing(false);
  };

  const handleDeleteClick = () => setShowDeleteConfirm(true);
  const handleDeleteConfirm = async () => {
    setShowDeleteConfirm(false);
    setDeleting(true);
    try {
      if (onDelete) {
        onDelete(node.id);
      }
    } finally {
      setDeleting(false);
    }
  };
  const handleDeleteCancel = () => setShowDeleteConfirm(false);

  return (
    <div className="info-panel">
      <div className="info-panel-header">
        <h3>Node Details</h3>
        <div className="header-actions">
        {!node.isStaleSnapshot && !editing && !showDeleteConfirm && (
          <>
            <button onClick={() => setEditing(true)} className="edit-btn">
              ✏️ Edit
            </button>
            {node.isVirtual && (
              <button onClick={handleDeleteClick} className="delete-btn" disabled={deleting}>
                {deleting ? '...' : '🗑️'}
              </button>
            )}
          </>
        )}
          {node.isVirtual && showDeleteConfirm && (
            <div className="delete-confirm-inline">
              <span>Delete &quot;{node.label}&quot;?</span>
              <button onClick={handleDeleteConfirm} className="confirm-delete-btn" disabled={deleting}>
                {deleting ? '...' : 'Delete'}
              </button>
              <button onClick={handleDeleteCancel} className="cancel-delete-btn" disabled={deleting}>
                Cancel
              </button>
            </div>
          )}
        </div>
      </div>

      {node.isStaleSnapshot && (
        <div className="info-section stale-notice">
          ⚠️ This node was in the snapshot but no longer exists in the code index. It is read-only.
        </div>
      )}

      <div className="info-section">
        <label>Name:</label>
        {editing && node.isVirtual ? (
          <input
            type="text"
            value={labelInput}
            onChange={(e) => setLabelInput(e.target.value)}
            placeholder="Node name"
            className="form-input"
          />
        ) : (
          <div className="info-value">{node.label}</div>
        )}
      </div>

      <div className="info-section">
        <label>Type:</label>
        <div className="info-value">
          {node.type}
          {node.isVirtual && <span className="virtual-badge">Virtual</span>}
        </div>
      </div>

      <div className="info-section">
        <label>File:</label>
        {editing && node.isVirtual ? (
          <>
            <input
              type="text"
              value={virtualDirInput}
              onChange={(e) => setVirtualDirInput(e.target.value)}
              placeholder="e.g., src/services/user"
              className="form-input"
            />
            <div className="field-hint">Directory path — controls placement in the file tree</div>
          </>
        ) : (
          <div className="info-value code">{node.filePath || <em>No file path</em>}</div>
        )}
      </div>

      {node.line > 0 && (
        <div className="info-section">
          <label>Line:</label>
          <div className="info-value">{node.line}</div>
        </div>
      )}

      <div className="info-section">
        <label>Description:</label>
        {editing ? (
          <textarea
            value={annotation.description || ''}
            onChange={(e) => setAnnotation({ ...annotation, description: e.target.value })}
            placeholder="Add a description..."
            rows={4}
          />
        ) : (
          <div className="info-value">
            {annotation.description || <em>No description</em>}
          </div>
        )}
      </div>

      <div className="info-section">
        <label>AI Remarks:</label>
        {editing ? (
          <textarea
            value={annotation.aiRemarks || ''}
            onChange={(e) => setAnnotation({ ...annotation, aiRemarks: e.target.value })}
            placeholder="Add AI-specific notes..."
            rows={4}
          />
        ) : (
          <div className="info-value">
            {annotation.aiRemarks || <em>No AI remarks</em>}
          </div>
        )}
      </div>

      {node.isVirtual && (
        <div className="info-section">
          <label>Code Snippet:</label>
          {editing ? (
            <textarea
              value={annotation.codeSnippet || ''}
              onChange={(e) => setAnnotation({ ...annotation, codeSnippet: e.target.value })}
              placeholder="Add code snippet..."
              rows={6}
              className="code-textarea"
            />
          ) : (
            <pre className="code-snippet">
              {annotation.codeSnippet || <em>No code snippet</em>}
            </pre>
          )}
        </div>
      )}

      {editing && (
        <div className="info-actions">
          <button onClick={handleSave} disabled={saving} className="save-btn">
            {saving ? 'Saving...' : 'Save'}
          </button>
          <button onClick={handleCancel} disabled={saving} className="cancel-btn">
            Cancel
          </button>
        </div>
      )}
    </div>
  );
}

interface EdgeInfoProps {
  edge: GraphEdge;
  isVirtual?: boolean;
  onUpdate?: () => void;
  onDelete?: (edgeId: string) => void;
}

function EdgeInfo({ edge, isVirtual, onUpdate, onDelete }: EdgeInfoProps) {
  const [editing, setEditing] = useState(false);
  const [annotation, setAnnotation] = useState<EdgeAnnotation>(edge.annotation || {});
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Reset annotation state when edge changes
  useEffect(() => {
    setAnnotation(edge.annotation || {});
    setEditing(false);  // Exit edit mode when switching edges
    setShowDeleteConfirm(false);
  }, [edge.id, edge.annotation]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await graphApi.updateEdgeAnnotationDirect(edge.id, annotation);
      setEditing(false);
      if (onUpdate) onUpdate();
    } catch (error) {
      console.error('Failed to save annotation:', error);
      postMessage({ type: 'showError', text: 'Failed to save annotation' });
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    // Reset to original annotation on cancel
    setAnnotation(edge.annotation || {});
    setEditing(false);
  };

  const handleDeleteClick = () => setShowDeleteConfirm(true);
  const handleDeleteConfirm = async () => {
    setShowDeleteConfirm(false);
    setDeleting(true);
    try {
      if (onDelete) {
        onDelete(edge.id);
      }
    } finally {
      setDeleting(false);
    }
  };
  const handleDeleteCancel = () => setShowDeleteConfirm(false);

  return (
    <div className="info-panel">
      <div className="info-panel-header">
        <h3>Edge Details</h3>
        <div className="header-actions">
          {!edge.isStaleSnapshot && !editing && !showDeleteConfirm && (
            <>
              <button onClick={() => setEditing(true)} className="edit-btn">
                ✏️ Edit
              </button>
              {isVirtual && (
                <button onClick={handleDeleteClick} className="delete-btn" disabled={deleting}>
                  {deleting ? '...' : '🗑️'}
                </button>
              )}
            </>
          )}
          {!edge.isStaleSnapshot && isVirtual && showDeleteConfirm && (
            <div className="delete-confirm-inline">
              <span>Delete this virtual edge?</span>
              <button onClick={handleDeleteConfirm} className="confirm-delete-btn" disabled={deleting}>
                {deleting ? '...' : 'Delete'}
              </button>
              <button onClick={handleDeleteCancel} className="cancel-delete-btn" disabled={deleting}>
                Cancel
              </button>
            </div>
          )}
        </div>
      </div>

      {edge.isStaleSnapshot && (
        <div className="info-section stale-notice">
          ⚠️ This edge was in the snapshot but no longer exists in the code index. It is read-only.
        </div>
      )}

      <div className="info-section">
        <label>Source:</label>
        <div className="info-value code">{edge.source}</div>
      </div>

      <div className="info-section">
        <label>Target:</label>
        <div className="info-value code">{edge.target}</div>
      </div>

      <div className="info-section">
        <label>Call Count:</label>
        <div className="info-value">
          {edge.count}
          {isVirtual && <span className="virtual-badge">Virtual</span>}
        </div>
      </div>

      <div className="info-section">
        <label>Remarks:</label>
        {editing ? (
          <textarea
            value={annotation.remarks || ''}
            onChange={(e) => setAnnotation({ ...annotation, remarks: e.target.value })}
            placeholder="Add remarks about this relationship..."
            rows={6}
          />
        ) : (
          <div className="info-value">
            {annotation.remarks || <em>No remarks</em>}
          </div>
        )}
      </div>

      {editing && (
        <div className="info-actions">
          <button onClick={handleSave} disabled={saving} className="save-btn">
            {saving ? 'Saving...' : 'Save'}
          </button>
          <button onClick={handleCancel} disabled={saving} className="cancel-btn">
            Cancel
          </button>
        </div>
      )}
    </div>
  );
}
