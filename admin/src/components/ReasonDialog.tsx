import { useEffect, useId, useRef, useState } from 'react';
import { useFocusTrap } from '../hooks/useFocusTrap';

interface ReasonDialogProps {
  readonly open: boolean;
  readonly title: string;
  readonly message: string;
  readonly label: string;
  readonly hint?: string;
  readonly placeholder?: string;
  readonly confirmLabel: string;
  readonly busyLabel?: string;
  readonly variant?: 'danger' | 'default';
  readonly busy?: boolean;
  readonly error?: string;
  readonly onClose: () => void;
  readonly onConfirm: (reason: string) => void;
}

const MAX_REASON_LENGTH = 1000;

/**
 * Collects a written reason before an action that another person will read
 * about: a rejection, or the retraction of a published version.
 *
 * The reason is mandatory because the alternative — a status change with no
 * explanation — leaves the recipient unable to act on it.
 */
export default function ReasonDialog({
  open,
  title,
  message,
  label,
  hint,
  placeholder,
  confirmLabel,
  busyLabel = 'Working…',
  variant = 'danger',
  busy = false,
  error = '',
  onClose,
  onConfirm,
}: ReasonDialogProps) {
  const dialogId = useId();
  const dialogRef = useRef<HTMLDivElement>(null);
  const [reason, setReason] = useState('');

  useFocusTrap(dialogRef, open);

  useEffect(() => {
    if (!open) {
      setReason('');
      return;
    }
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !busy) onClose();
    };
    globalThis.addEventListener('keydown', handleKeyDown);
    return () => globalThis.removeEventListener('keydown', handleKeyDown);
  }, [busy, onClose, open]);

  if (!open) return null;

  const trimmed = reason.trim();
  const tooLong = trimmed.length > MAX_REASON_LENGTH;
  const canConfirm = trimmed.length > 0 && !tooLong && !busy;

  return (
    <div className="modal-backdrop" onClick={() => { if (!busy) onClose(); }}>
      <div
        ref={dialogRef}
        className={variant === 'danger' ? 'modal modal--danger' : 'modal'}
        role="dialog"
        aria-modal="true"
        aria-labelledby={`${dialogId}-title`}
        aria-describedby={`${dialogId}-description`}
        onClick={(event) => event.stopPropagation()}
      >
        <h2 id={`${dialogId}-title`} className="modal__title">{title}</h2>
        <p id={`${dialogId}-description`} className="modal__description">{message}</p>

        <div className="field">
          <label htmlFor={`${dialogId}-reason`}>{label}</label>
          <textarea
            id={`${dialogId}-reason`}
            className="input"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder={placeholder}
            maxLength={MAX_REASON_LENGTH}
            rows={4}
            aria-describedby={`${dialogId}-hint`}
            aria-invalid={tooLong || undefined}
            autoFocus
            required
          />
          <span id={`${dialogId}-hint`} className="field__hint">
            {hint ?? 'This is shown to the plugin author.'} {trimmed.length}/{MAX_REASON_LENGTH}
          </span>
        </div>

        {error && <div className="alert alert--error mt-1" role="alert">{error}</div>}

        <div className="modal__actions">
          <button type="button" className="btn" onClick={onClose} disabled={busy}>
            Cancel
          </button>
          <button
            type="button"
            className={variant === 'danger' ? 'btn btn--danger' : 'btn btn--primary'}
            onClick={() => onConfirm(trimmed)}
            disabled={!canConfirm}
          >
            {busy ? busyLabel : confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
