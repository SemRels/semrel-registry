import { useTheme } from '../hooks/useTheme';
import type { ThemePreference } from '../hooks/useTheme';

const LABELS: Record<ThemePreference, { icon: string; label: string; next: string }> = {
  system: { icon: '🖥', label: 'System theme', next: 'light theme' },
  light:  { icon: '☀', label: 'Light theme',  next: 'dark theme' },
  dark:   { icon: '🌙', label: 'Dark theme',   next: 'system theme' },
};

/**
 * Cycles system → light → dark.
 *
 * The icon is aria-hidden and the accessible name spells out both the current
 * state and what pressing it does, because an icon alone tells a screen-reader
 * user neither.
 */
export default function ThemeToggle({ className }: { readonly className?: string }) {
  const { preference, cycle } = useTheme();
  const current = LABELS[preference];

  return (
    <button
      type="button"
      className={className ? `theme-toggle ${className}` : 'theme-toggle'}
      onClick={cycle}
      aria-label={`${current.label}. Switch to ${current.next}.`}
      title={`${current.label} — click for ${current.next}`}
    >
      <span aria-hidden="true">{current.icon}</span>
      <span>{current.label.replace(' theme', '')}</span>
    </button>
  );
}
