// Snapshots dropdown component for viewing and managing saved snapshots

import { useState, useEffect } from 'react';
import { Snapshot } from '../../types/api';
import { graphApi } from '../../services/graphApi';
import { postMessage } from '../../services/webviewTransport';
import { PromptPreviewModal } from '../PromptPreviewModal';
import './SnapshotsDropdown.css';

interface SnapshotsDropdownProps {
  isOpen: boolean;
  onClose: () => void;
  onLoadSnapshot?: (id: string) => void;
}

export function SnapshotsDropdown({ isOpen, onClose, onLoadSnapshot }: SnapshotsDropdownProps) {
  const [snapshots, setSnapshots] = useState<Snapshot[]>([]);
  const [loading, setLoading] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [previewSnapshot, setPreviewSnapshot] = useState<Snapshot | null>(null);
  const [deleteConfirmId, setDeleteConfirmId] = useState<string | null>(null);

  // Load snapshots when dropdown opens
  useEffect(() => {
    if (isOpen) {
      loadSnapshots();
    }
  }, [isOpen]);

  const loadSnapshots = async () => {
    setLoading(true);
    try {
      const data = await graphApi.listSnapshots();
      setSnapshots(data || []);
    } catch (error) {
      console.error('Failed to load snapshots:', error);
      setSnapshots([]);
    } finally {
      setLoading(false);
    }
  };

  const handleCopyId = async (id: string) => {
    try {
      await navigator.clipboard.writeText(id);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch (error) {
      // Fallback for older browsers
      const textArea = document.createElement('textarea');
      textArea.value = id;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    }
  };

  const handleDeleteClick = (id: string) => {
    setDeleteConfirmId(id);
  };

  const handleDeleteConfirm = async (id: string) => {
    setDeleteConfirmId(null);
    try {
      await graphApi.deleteSnapshot(id);
      setSnapshots(prev => prev.filter(s => s.id !== id));
    } catch (error) {
      console.error('Failed to delete snapshot:', error);
      postMessage({ type: 'showError', text: 'Failed to delete snapshot' });
    }
  };

  const handleDeleteCancel = () => {
    setDeleteConfirmId(null);
  };

  const formatDate = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  if (!isOpen) {
    return null;
  }

  return (
    <>
      <div className="snapshots-overlay" onClick={onClose} />
      <div className="snapshots-dropdown">
        <div className="snapshots-header">
          <h3>Saved Snapshots</h3>
          <button className="snapshots-close-btn" onClick={onClose}>
            ✕
          </button>
        </div>

        <div className="snapshots-list">
          {loading ? (
            <div className="snapshots-empty">Loading...</div>
          ) : snapshots.length === 0 ? (
            <div className="snapshots-empty">
              No snapshots yet.<br />
              Use "Capture Snapshot" in the Info Panel to save your graph state.
            </div>
          ) : (
            snapshots.map(snapshot => (
              <div key={snapshot.id} className="snapshot-item">
                <div className="snapshot-item-header">
                  <span className="snapshot-name">
                    {snapshot.name || 'Untitled Snapshot'}
                  </span>
                  <span className="snapshot-id">{snapshot.id}</span>
                </div>

                <div className="snapshot-meta">
                  <span>🎯 {snapshot.virtualNodeCount} virtual</span>
                  <span>📄 {snapshot.contextNodeCount} context</span>
                  <span>🔗 {snapshot.edgeCount} edges</span>
                </div>

                <div className="snapshot-meta">
                  <span>📅 {formatDate(snapshot.createdAt)}</span>
                </div>

                <div className="snapshot-actions">
                  {deleteConfirmId === snapshot.id ? (
                    <div className="snapshot-delete-confirm">
                      <span>Delete?</span>
                      <button
                        className="snapshot-btn snapshot-btn-confirm-delete"
                        onClick={() => handleDeleteConfirm(snapshot.id)}
                      >
                        Delete
                      </button>
                      <button
                        className="snapshot-btn snapshot-btn-cancel"
                        onClick={handleDeleteCancel}
                      >
                        Cancel
                      </button>
                    </div>
                  ) : (
                    <>
                      <button
                        className="snapshot-btn snapshot-btn-preview"
                        onClick={() => setPreviewSnapshot(snapshot)}
                      >
                        Preview
                      </button>
                      {onLoadSnapshot && (
                        <button
                          className="snapshot-btn snapshot-btn-load"
                          onClick={() => { onLoadSnapshot(snapshot.id); onClose(); }}
                        >
                          Load
                        </button>
                      )}
                      <button
                        className={`snapshot-btn snapshot-btn-copy ${copiedId === snapshot.id ? 'copied' : ''}`}
                        onClick={() => handleCopyId(snapshot.id)}
                      >
                        {copiedId === snapshot.id ? '✓ Copied!' : '📋 Copy ID'}
                      </button>
                      <button
                        className="snapshot-btn snapshot-btn-delete"
                        onClick={() => handleDeleteClick(snapshot.id)}
                      >
                        🗑️
                      </button>
                    </>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Prompt Preview Modal */}
      <PromptPreviewModal
        snapshotId={previewSnapshot?.id || ''}
        snapshotName={previewSnapshot?.name}
        isOpen={!!previewSnapshot}
        onClose={() => setPreviewSnapshot(null)}
      />
    </>
  );
}
