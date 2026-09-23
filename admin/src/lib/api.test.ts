import { afterEach, describe, expect, it, vi } from 'vitest';
import { getAuthConfig, getCurrentUser, getToken, saveToken, validatePlugin } from './api';

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
  localStorage.clear();
});

describe('validatePlugin', () => {
  it('resolves with the validation result on success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      jsonResponse({ valid: true, checks: [], summary: 'ok' }),
    ));

    const result = await validatePlugin('https://github.com/acme/analyzer-x');

    expect(result).toEqual({ valid: true, checks: [], summary: 'ok' });
  });

  // The API can reject the request for reasons that have nothing to do with
  // the repository being validated — an origin check, a rate limit, a 500 —
  // and the error body it sends back does not have a "checks" array. A caller
  // that trusted the response unconditionally crashed doing `checks.map(...)`
  // on undefined instead of showing the error it was actually given.
  it('throws instead of resolving with a body that has no checks array', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      jsonResponse({ error: 'Request origin is not allowed to perform this action' }, 403),
    ));

    await expect(validatePlugin('https://github.com/acme/analyzer-x')).rejects.toThrow();
  });
});

describe('getAuthConfig', () => {
  it('resolves with the auth config on success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      jsonResponse({ githubOAuthEnabled: true, loginURL: '/auth/github' }),
    ));

    const result = await getAuthConfig();

    expect(result).toEqual({ githubOAuthEnabled: true, loginURL: '/auth/github' });
  });

  it('throws rather than resolving with a malformed body on failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      jsonResponse({ error: 'internal error' }, 500),
    ));

    await expect(getAuthConfig()).rejects.toThrow();
  });
});

describe('getCurrentUser', () => {
  // A stale dev token (leftover from the ADMIN_TOKEN login form) takes
  // precedence over a valid GitHub session cookie, since the API checks the
  // Authorization header before the cookie. Regression test for a bug where
  // this call never cleared the bad token, permanently locking the browser
  // out of an otherwise-valid cookie session.
  it('clears a stale dev token that the API rejected', async () => {
    saveToken('stale-token');
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ error: 'invalid token' }, 401)));

    const result = await getCurrentUser();

    expect(result).toBeNull();
    expect(getToken()).toBe('');
  });

  it('does not touch storage when there was no token to begin with', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse({ error: 'no session' }, 401)));

    const result = await getCurrentUser();

    expect(result).toBeNull();
    expect(getToken()).toBe('');
  });

  it('resolves the user on success', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(
      jsonResponse({ user: { login: 'mwaldheim', is_admin: true } }),
    ));

    const result = await getCurrentUser();

    expect(result).toEqual({ login: 'mwaldheim', name: '', avatarUrl: '', role: 'admin', isAdmin: true });
  });
});
