import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { verifyToken, saveToken, getAuthConfig } from '../lib/api';
import type { AuthConfig } from '../lib/api';
import LegalLinks from '../components/LegalLinks';

export default function LoginPage() {
  const [token, setToken]   = useState('');
  const [error, setError]   = useState('');
  const [loading, setLoading] = useState(false);
  const [cfg, setCfg]       = useState<AuthConfig | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    getAuthConfig().then(setCfg).catch(() => null);
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(''); setLoading(true);
    try {
      if (await verifyToken(token.trim())) {
        saveToken(token.trim());
        navigate('/admin');
      } else {
        setError('Invalid token.');
      }
    } catch {
      setError('Cannot connect to API (:8080).');
    } finally { setLoading(false); }
  }

  return (
    <div className="login-wrap">
      <div className="login-card">
        <div style={{ display:"flex", flexDirection:"column", alignItems:"center", gap:".5rem", marginBottom:"1.5rem" }}>
          <img src="/semrel.svg" alt="semrel" style={{ width:"3rem", height:"3rem" }} />
          <h1 style={{ margin:0, textAlign:"center" }}>semrel Registry</h1>
        </div>

        {cfg?.githubOAuthEnabled ? (
          <>
            <a
              href={cfg.loginURL}
              className="btn btn--primary"
              style={{ width: '100%', justifyContent: 'center' }}
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
            {error && <div className="alert alert--error">{error}</div>}
            <form onSubmit={(e) => { void handleSubmit(e); }}>
              <div className="field">
                <label htmlFor="token">Admin token</label>
                <input id="token" type="password" className="input" value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder="dev-secret" autoFocus required />
              </div>
              <button type="submit" className="btn btn--primary" style={{ width: '100%' }}
                disabled={loading || !token.trim()}>
                {loading ? 'Checking…' : 'Sign in with token'}
              </button>
            </form>
            <p className="legal-note mt-1">
              By signing in, you agree to the semrel registry terms for authenticated contributors and acknowledge the applicable privacy and publisher information.
            </p>
            <LegalLinks inline className="legal-note__links" linkClassName="muted" />
            <p className="muted mt-1" style={{ fontSize: 'var(--fs-xs)' }}>
              Set <code>GITHUB_CLIENT_ID</code> + <code>GITHUB_CLIENT_SECRET</code> to enable GitHub OAuth.
            </p>
          </>
        )}
      </div>
    </div>
  );
}
