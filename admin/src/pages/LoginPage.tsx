import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { verifyToken, saveToken } from '../lib/api';

export default function LoginPage() {
  const [token, setToken]   = useState('');
  const [error, setError]   = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(''); setLoading(true);
    try {
      if (await verifyToken(token.trim())) {
        saveToken(token.trim());
        navigate('/');
      } else {
        setError('Invalid token. Check the ADMIN_TOKEN env variable.');
      }
    } catch {
      setError('Cannot connect to API. Is the Go server running on :8080?');
    } finally { setLoading(false); }
  }

  return (
    <div className="login-wrap">
      <div className="login-card">
        <h1>registry.semrel.io — Admin</h1>
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
            {loading ? 'Checking…' : 'Sign in'}
          </button>
        </form>
        <p className="muted mt-1" style={{ fontSize: 'var(--fs-xs)' }}>
          Default dev token: <code>dev-secret</code>
        </p>
      </div>
    </div>
  );
}
