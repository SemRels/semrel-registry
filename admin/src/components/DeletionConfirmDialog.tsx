import { useEffect, useId, useState } from 'react';

interface DeletionConfirmDialogProps {
  open: boolean;
  title: string;
  message: string;
  confirmationValue: string;
  confirmationLabel: string;
  confirmLabel: string;
  busyLabel?: string;
  acknowledgement?: string;
  busy?: boolean;
  error?: string;
  onClose: () => void;
  onConfirm: () => void;
}

export default function DeletionConfirmDialog({
  open,
  title,
  message,
  confirmationValue,
  confirmationLabel,
  confirmLabel,
  busyLabel = 'Deleting…',
  acknowledgement = 'I understand this action cannot be undone.',
  busy = false,
  error = '',
  onClose,
  onConfirm,
}: Readonly<DeletionConfirmDialogProps>) {
  const dialogId = useId();
  const [typedValue, setTypedValue] = useState('');
  const [acknowledged, setAcknowledged] = useState(false);

  useEffect(() => {
    if (!open) {
      setTypedValue('');
      setAcknowledged(false);
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !busy) onClose();
    };

    globalThis.addEventListener('keydown', handleKeyDown);
    return () => globalThis.removeEventListener('keydown', handleKeyDown);
  }, [busy, onClose, open]);

  if (!open) return null;

  const canConfirm = acknowledged && typedValue.trim() === confirmationValue;

  return (
    <div className="modal-backdrop" onClick={() => { if (!busy) onClose(); }}>
      <div
        className="modal modal--danger"
        role="dialog"
        aria-modal="true"
        aria-labelledby={`${dialogId}-title`}
        aria-describedby={`${dialogId}-description`}
        onClick={(event) => event.stopPropagation()}
      >
        <h2 id={`${dialogId}-title`} className="modal__title">{title}</h2>
        <p id={`${dialogId}-description`} className="modal__description">{message}</p>

        <div className="alert alert--info" style={{ marginBottom: '1rem' }}>
          Type <code>{confirmationValue}</code> to continue.
        </div>

        <div className="field">
          <label htmlFor={`${dialogId}-confirmation`}>{confirmationLabel}</label>
          <input
            id={`${dialogId}-confirmation`}
            className="input"
            value={typedValue}
            onChange={(event) => setTypedValue(event.target.value)}
            autoComplete="off"
            spellCheck={false}
            autoFocus
          />
        </div>

        <label className="checkbox-row" htmlFor={`${dialogId}-acknowledgement`}>
          <input
            id={`${dialogId}-acknowledgement`}
            type="checkbox"
            checked={acknowledged}
            onChange={(event) => setAcknowledged(event.target.checked)}
          />
          <span>{acknowledgement}</span>
        </label>

        {error && <div className="alert alert--error mt-1">{error}</div>}

        <div className="modal__actions">
          <button type="button" className="btn" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button
            type="button"
            className="btn btn--danger"
            onClick={onConfirm}
            disabled={!canConfirm || busy}
          >
            {busy ? busyLabel : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
