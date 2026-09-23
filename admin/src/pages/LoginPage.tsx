import { useEffect, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { verifyToken, saveToken, getAuthConfig } from '../lib/api';
import type { AuthConfig } from '../lib/api';
import LegalLinks from '../components/LegalLinks';
import ThemeToggle from '../components/ThemeToggle';

interface LoginState {
  /** Set when the user was bounced here by an expired session. */
  expired?: boolean;
  /** The page to return to after signing in. */
  from?: string;
}

export default function LoginPage() {
  const [token, setToken]     = useState('');
  const [error, setError]     = useState('');
  const [loading, setLoading] = useState(false);
  const [cfg, setCfg]         = useState<AuthConfig | null>(null);
  const navigate = useNavigate();
  const location = useLocation();
  const state = (location.state ?? {}) as LoginState;
  const returnTo = state.from && state.from.startsWith('/') ? state.from : '/admin';

  useEffect(() => {
    getAuthConfig().then(setCfg).catch(() => null);
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      if (await verifyToken(token.trim())) {
        saveToken(token.trim());
        navigate(returnTo, { replace: true });
      } else {
        setError('That token was not accepted. Check it and try again.');
      }
    } catch {
      setError('Could not reach the registry API. Is it running?');
    } finally {
      setLoading(false);
    }
  }

  // Carry the intended destination through the OAuth round trip; the API
  // validates it as a same-site path before redirecting back.
  const signInURL = cfg ? `${cfg.loginURL}?next=${encodeURIComponent(returnTo)}` : '#';

  return (
    <main className="login-wrap" id="main-content">
      <div className="login-card">
        <div className="login-card__header">
          <img src="/semrel.svg" alt="" aria-hidden="true" className="login-card__logo" />
          <h1 className="login-card__title">semrel Registry</h1>
        </div>

        {/* Explains why the user is here rather than dropping them at a bare
            login form after a silent redirect. */}
        {state.expired && (
          <div className="alert alert--info" role="status">
            Your session has expired. Sign in again to continue where you left off.
          </div>
        )}

        {cfg?.githubOAuthEnabled ? (
          <>
            <a
              href={signInURL}
              className="btn btn--primary btn--full"
            >
              Sign in with GitHub
            </a>
            <p className="legal-note mt-1">
              By signing in, you agree to the semrel registry terms for authenticated contributors and acknowledge the applicable privacy and publisher information.
            </p>
            <LegalLinks inline className="legal-note__links" linkClassName="muted" />
          </>
        ) : (
          <>
            {/* role="alert" so the failure is announced, not just repainted. */}
            {error && <div className="alert alert--error" role="alert">{error}</div>}
            <form onSubmit={(e) => { void handleSubmit(e); }}>
              <div className="field">
                <label htmlFor="token">Admin token</label>
                <input
                  id="token"
                  type="password"
                  className="input"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder="dev-secret"
                  autoComplete="current-password"
                  aria-describedby={error ? 'token-error' : 'token-hint'}
                  aria-invalid={error ? true : undefined}
                  autoFocus
                  required
                />
                {error
                  ? <span id="token-error" className="field__error">{error}</span>
                  : <span id="token-hint" className="field__hint">Development fallback. Production uses GitHub sign-in.</span>}
              </div>
              <button type="submit" className="btn btn--primary btn--full"
                disabled={loading || !token.trim()}>
                {loading ? 'Checking…' : 'Sign in with token'}
              </button>
            </form>
            <p className="legal-note mt-1">
              By signing in, you agree to the semrel registry terms for authenticated contributors and acknowledge the applicable privacy and publisher information.
            </p>
            <LegalLinks inline className="legal-note__links" linkClassName="muted" />
            <p className="muted mt-1 text-xs">
              Set <code>GITHUB_CLIENT_ID</code> + <code>GITHUB_CLIENT_SECRET</code> to enable GitHub OAuth.
            </p>
          </>
        )}

        <div className="flex justify-center mt-2">
          <ThemeToggle />
        </div>
      </div>
    </main>
  );
}
