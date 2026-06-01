import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { verifyToken, saveToken, getAuthConfig } from '../lib/api';
import type { AuthConfig } from '../lib/api';

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
        <div style={{ display:"flex", alignItems:"center", gap:".5rem", marginBottom:"1rem" }}><img src="/semrel.svg" alt="semrel" style={{ width:"1.75rem", height:"1.75rem" }} /><h1 style={{ margin:0 }}>semrel Registry</h1></div>

        {cfg?.githubOAuthEnabled && (
          <>
            <a
              href={cfg.loginURL}
              className="btn btn--primary"
              style={{ width: '100%', justifyContent: 'center', marginBottom: '.75rem' }}
            >
              Sign in with GitHub
            </a>
            <div style={{ display:'flex', alignItems:'center', gap:'.5rem', marginBottom:'.75rem' }}>
              <hr style={{ flex:1, border:'none', borderTop:'1px solid var(--border)' }} />
              <span className="muted" style={{ fontSize:'var(--fs-xs)', whiteSpace:'nowrap' }}>or use dev token</span>
              <hr style={{ flex:1, border:'none', borderTop:'1px solid var(--border)' }} />
            </div>
          </>
        )}

        {error && <div className="alert alert--error">{error}</div>}

        <form onSubmit={(e) => { void handleSubmit(e); }}>
          <div className="field">
            <label htmlFor="token">Admin token</label>
            <input id="token" type="password" className="input" value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="dev-secret" autoFocus={!cfg?.githubOAuthEnabled} required />
          </div>
          <button type="submit" className="btn btn--primary" style={{ width: '100%' }}
            disabled={loading || !token.trim()}>
            {loading ? 'Checking…' : 'Sign in with token'}
          </button>
        </form>

        {!cfg?.githubOAuthEnabled && (
          <p className="muted mt-1" style={{ fontSize: 'var(--fs-xs)' }}>
            Set <code>GITHUB_CLIENT_ID</code> + <code>GITHUB_CLIENT_SECRET</code> in <code>api/.env</code> to enable GitHub OAuth.
          </p>
        )}
      </div>
    </div>
  );
}
