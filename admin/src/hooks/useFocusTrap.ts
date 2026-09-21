import { useEffect, type RefObject } from 'react';

const FOCUSABLE = [
  'a[href]', 'button:not([disabled])', 'input:not([disabled])',
  'select:not([disabled])', 'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

/**
 * Confines Tab navigation to `container` while `active`, and restores focus to
 * whatever was focused before.
 *
 * A drawer that visually covers the page but leaves the rest of the document
 * tabbable is a trap of the opposite kind: keyboard users tab into content they
 * cannot see and lose track of where they are.
 */
export function useFocusTrap(container: RefObject<HTMLElement | null>, active: boolean) {
  useEffect(() => {
    if (!active || !container.current) return;

    const element = container.current;
    const previouslyFocused = document.activeElement as HTMLElement | null;

    const focusable = () => Array.from(element.querySelectorAll<HTMLElement>(FOCUSABLE))
      .filter(node => node.offsetParent !== null);

    focusable()[0]?.focus();

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Tab') return;
      const nodes = focusable();
      if (nodes.length === 0) return;

      const first = nodes[0];
      const last = nodes[nodes.length - 1];

      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      // Returning focus is what lets a keyboard user carry on from where they
      // opened the drawer instead of restarting at the top of the document.
      previouslyFocused?.focus?.();
    };
  }, [container, active]);
}
