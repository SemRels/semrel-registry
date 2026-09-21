import { useCallback, useEffect, useState } from 'react';

export type ThemePreference = 'system' | 'light' | 'dark';

const STORAGE_KEY = 'semrel_theme';

function readStoredPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored === 'light' || stored === 'dark' || stored === 'system') return stored;
  } catch {
    // Private browsing and "block site data" both make localStorage throw.
  }
  return 'system';
}

/**
 * Applies the preference to the document root.
 *
 * "system" removes the attribute entirely rather than writing a resolved value,
 * so the stylesheet's prefers-color-scheme rules stay in charge and the page
 * follows the OS if the visitor changes it while the tab is open.
 */
function applyPreference(preference: ThemePreference) {
  const root = document.documentElement;
  if (preference === 'system') {
    root.removeAttribute('data-theme');
  } else {
    root.setAttribute('data-theme', preference);
  }
}

/** Reads and persists the visitor's colour-scheme preference. */
export function useTheme() {
  const [preference, setPreference] = useState<ThemePreference>(readStoredPreference);

  useEffect(() => {
    applyPreference(preference);
    try {
      localStorage.setItem(STORAGE_KEY, preference);
    } catch {
      // Not being able to remember the choice is not worth failing over.
    }
  }, [preference]);

  const cycle = useCallback(() => {
    setPreference(current => {
      if (current === 'system') return 'light';
      if (current === 'light') return 'dark';
      return 'system';
    });
  }, []);

  return { preference, setPreference, cycle };
}

/** Applies the stored preference before React renders, to avoid a flash. */
export function initTheme() {
  applyPreference(readStoredPreference());
}
