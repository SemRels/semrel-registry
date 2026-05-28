import { useEffect, useState } from 'react';
import { getToken } from '../lib/api';

export interface CurrentUser {
  login: string;
  name: string;
  avatarUrl: string;
  role: string;       // "admin" | "user"
  isAdmin: boolean;
}

function parseJWT(token: string): CurrentUser | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    const payload = JSON.parse(atob(parts[1]));
    return {
      login:     payload.login     ?? '',
      name:      payload.name      ?? '',
      avatarUrl: payload.avatar_url ?? '',
      role:      payload.role      ?? 'user',
      isAdmin:   payload.is_admin  === true || payload.role === 'admin',
    };
  } catch {
    return null;
  }
}

/** Returns the current user decoded from the stored JWT, or null if not logged in. */
export function useCurrentUser(): CurrentUser | null {
  const [user, setUser] = useState<CurrentUser | null>(null);

  useEffect(() => {
    const token = getToken();
    if (!token) return;
    // Static ADMIN_TOKEN (dev): cannot decode; treat as admin.
    if (!token.includes('.')) {
      setUser({ login: 'admin', name: 'Dev Admin', avatarUrl: '', role: 'admin', isAdmin: true });
      return;
    }
    setUser(parseJWT(token));
  }, []);

  return user;
}
