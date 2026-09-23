import { useCallback, useEffect, useState } from 'react';
import { getCurrentUser, onSessionExpired } from '../lib/api';
import type { SessionUser } from '../lib/api';

export type CurrentUser = SessionUser;

export interface CurrentUserState {
  user: CurrentUser | null;
  /** True until the first /auth/me response arrives. */
  loading: boolean;
  refresh: () => void;
}

/**
 * Resolves the signed-in user from the API.
 *
 * This used to decode the JWT out of localStorage, which only worked because
 * the token was readable by any script on the page. The session is now an
 * HttpOnly cookie, so identity has to be asked for — which also means an
 * expired or revoked session is noticed instead of being trusted until the
 * next failed write.
 */
export function useCurrentUser(): CurrentUserState {
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [loading, setLoading] = useState(true);
  const [nonce, setNonce] = useState(0);

  const refresh = useCallback(() => setNonce(n => n + 1), []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    getCurrentUser()
      .then(result => {
        if (!cancelled) setUser(result);
      })
      .catch(() => {
        if (!cancelled) setUser(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => { cancelled = true; };
  }, [nonce]);

  // A session that expires mid-visit must clear the cached identity, otherwise
  // the UI keeps offering actions that will fail.
  useEffect(() => onSessionExpired(() => setUser(null)), []);

  return { user, loading, refresh };
}
