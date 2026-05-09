// Modal component for previewing and editing snapshot prompts

import { useState, useEffect, useCallback } from 'react';
import { graphApi } from '../../services/graphApi';
import './PromptPreviewModal.css';

interface PromptPreviewModalProps {
  snapshotId: string;
  snapshotName?: string;
  isOpen: boolean;
  onClose: () => void;
}

export function PromptPreviewModal({
  snapshotId,
  snapshotName,
  isOpen,
  onClose,
}: PromptPreviewModalProps) {
  const [prompt, setPrompt] = useState('');
  const [originalPrompt, setOriginalPrompt] = useState('');
  const [isCustom, setIsCustom] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [copied, setCopied] = useState(false);
  const [status, setStatus] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  // Load prompt when modal opens
  useEffect(() => {
    if (isOpen && snapshotId) {
      loadPrompt();
    }
  }, [isOpen, snapshotId]);

  const loadPrompt = async () => {
    setLoading(true);
    setStatus(null);
    try {
      const response = await graphApi.getSnapshotPrompt(snapshotId);
      setPrompt(response.prompt);
      setOriginalPrompt(response.prompt);
      setIsCustom(response.isCustom);
    } catch (error) {
      console.error('Failed to load prompt:', error);
      setStatus({ type: 'error', message: 'Failed to load prompt' });
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async () => {
    if (!prompt.trim()) {
      setStatus({ type: 'error', message: 'Prompt cannot be empty' });
      return;
    }

    setSaving(true);
    setStatus(null);
    try {
      await graphApi.updateSnapshotPrompt(snapshotId, prompt);
      setOriginalPrompt(prompt);
      setIsCustom(true);
      setStatus({ type: 'success', message: 'Saved!' });
      setTimeout(() => setStatus(null), 2000);
    } catch (error) {
      console.error('Failed to save prompt:', error);
      setStatus({ type: 'error', message: 'Failed to save' });
    } finally {
      setSaving(false);
    }
  };

  const handleReset = async () => {
    if (!confirm('Reset to auto-generated prompt? Your custom edits will be lost.')) {
      return;
    }

    setSaving(true);
    setStatus(null);
    try {
      const response = await graphApi.resetSnapshotPrompt(snapshotId);
      setPrompt(response.prompt);
      setOriginalPrompt(response.prompt);
      setIsCustom(false);
      setStatus({ type: 'success', message: 'Reset to generated prompt' });
      setTimeout(() => setStatus(null), 2000);
    } catch (error) {
      console.error('Failed to reset prompt:', error);
      setStatus({ type: 'error', message: 'Failed to reset' });
    } finally {
      setSaving(false);
    }
  };

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(prompt);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (error) {
      // Fallback for older browsers
      const textArea = document.createElement('textarea');
      textArea.value = prompt;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }, [prompt]);

  const hasChanges = prompt !== originalPrompt;

  const handleClose = () => {
    if (hasChanges) {
      if (!confirm('You have unsaved changes. Close anyway?')) {
        return;
      }
    }
    onClose();
  };

  // Handle escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        handleClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, hasChanges]);

  if (!isOpen) {
    return null;
  }

  return (
    <div className="prompt-modal-overlay" onClick={handleClose}>
      <div className="prompt-modal" onClick={(e) => e.stopPropagation()}>
        <div className="prompt-modal-header">
          <h2>
            Prompt Preview
            {snapshotName && ` - ${snapshotName}`}
            {isCustom && <span className="custom-badge">Custom</span>}
          </h2>
          <button className="prompt-modal-close" onClick={handleClose}>
            &times;
          </button>
        </div>

        <div className="prompt-modal-body">
          {loading ? (
            <div className="prompt-loading">Loading prompt...</div>
          ) : (
            <textarea
              className="prompt-textarea"
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="Prompt content..."
              spellCheck={false}
            />
          )}
        </div>

        <div className="prompt-modal-footer">
          <div className="prompt-modal-footer-left">
            <button
              className={`prompt-btn prompt-btn-secondary ${copied ? 'copied' : ''}`}
              onClick={handleCopy}
              disabled={loading}
            >
              {copied ? 'Copied!' : 'Copy'}
            </button>
            {isCustom && (
              <button
                className="prompt-btn prompt-btn-danger"
                onClick={handleReset}
                disabled={loading || saving}
              >
                Reset
              </button>
            )}
          </div>

          <div className="prompt-modal-footer-right">
            {status && (
              <span className={`prompt-status ${status.type}`}>
                {status.message}
              </span>
            )}
            <button
              className="prompt-btn prompt-btn-secondary"
              onClick={handleClose}
            >
              Close
            </button>
            <button
              className="prompt-btn prompt-btn-primary"
              onClick={handleSave}
              disabled={loading || saving || !hasChanges}
            >
              {saving ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
