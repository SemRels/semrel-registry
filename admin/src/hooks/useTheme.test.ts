import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useTheme, initTheme } from './useTheme';

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-theme');
  });

  afterEach(() => {
    document.documentElement.removeAttribute('data-theme');
  });

  it('defaults to following the system preference', () => {
    const { result } = renderHook(() => useTheme());
    expect(result.current.preference).toBe('system');
    // "system" must leave the attribute off so the stylesheet's
    // prefers-color-scheme rules stay in charge.
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
  });

  it('cycles system → light → dark → system', () => {
    const { result } = renderHook(() => useTheme());

    act(() => result.current.cycle());
    expect(result.current.preference).toBe('light');
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');

    act(() => result.current.cycle());
    expect(result.current.preference).toBe('dark');
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');

    act(() => result.current.cycle());
    expect(result.current.preference).toBe('system');
    expect(document.documentElement.hasAttribute('data-theme')).toBe(false);
  });

  it('remembers the choice across reloads', () => {
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setPreference('dark'));

    document.documentElement.removeAttribute('data-theme');
    initTheme();

    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');
  });

  it('survives storage being unavailable', () => {
    const original = Storage.prototype.getItem;
    Storage.prototype.getItem = () => { throw new Error('denied'); };
    try {
      expect(() => initTheme()).not.toThrow();
    } finally {
      Storage.prototype.getItem = original;
    }
  });
});
