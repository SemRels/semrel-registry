import { useEffect, useRef, useState } from 'react';

interface CopyButtonProps {
  readonly text: string;
  /** What is being copied, for the accessible name: "Copy install command". */
  readonly label?: string;
  readonly className?: string;
  readonly style?: React.CSSProperties;
}

/**
 * Copies text to the clipboard and confirms it.
 *
 * The confirmation is announced through a live region as well as shown: a
 * button whose label flicks to "Copied" for two seconds is invisible feedback
 * to anyone not looking at that exact spot.
 */
export default function CopyButton({ text, label = 'Copy', className, style }: CopyButtonProps) {
  const [state, setState] = useState<'idle' | 'copied' | 'failed'>('idle');
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => () => clearTimeout(timer.current), []);

  async function copy(event: React.MouseEvent) {
    // Cards make the whole surface a link; copying must not navigate.
    event.preventDefault();
    event.stopPropagation();

    try {
      await navigator.clipboard.writeText(text);
      setState('copied');
    } catch {
      // Clipboard access is denied in some browsers outside a secure context.
      setState('failed');
    }
    clearTimeout(timer.current);
    timer.current = setTimeout(() => setState('idle'), 2000);
  }

  const message = state === 'copied' ? 'Copied' : state === 'failed' ? 'Press Ctrl+C to copy' : label;

  return (
    <>
      <button
        type="button"
        className={className ?? 'copy-button'}
        style={style}
        onClick={(event) => { void copy(event); }}
        aria-label={`${label}: ${text}`}
      >
        <span aria-hidden="true">{state === 'copied' ? '✓ Copied' : label}</span>
      </button>
      <span className="sr-only" role="status" aria-live="polite">
        {state === 'idle' ? '' : message}
      </span>
    </>
  );
}
