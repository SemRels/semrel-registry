import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { verifyToken, saveToken } from '../lib/api';

export default function LoginPage() {
  const [token, setToken] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const ok = await verifyToken(token.trim());
      if (ok) {
        saveToken(token.trim());
        navigate('/');
      } else {
        setError('Invalid admin token. Check your ADMIN_TOKEN env variable.');
      }
    } catch {
      setError('Could not connect to the API. Is the Go server running?');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="login-page">
      <div className="login-card">
        <h1>🔐 Registry Admin</h1>
        <p>Enter your admin token to continue. The token is the value of the <code>ADMIN_TOKEN</code> environment variable set in the Go API.</p>

        {error && <div className="alert alert-error">{error}</div>}

        <form onSubmit={(e) => { void handleSubmit(e); }}>
          <div className="form-group">
            <label className="form-label" htmlFor="token">Admin Token</label>
            <input
              id="token"
              type="password"
              className="form-input"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="dev-secret"
              autoFocus
              required
            />
          </div>
          <button
            type="submit"
            className="btn btn-primary"
            style={{ width: '100%' }}
            disabled={loading || !token.trim()}
          >
            {loading ? 'Checking…' : 'Sign in →'}
          </button>
        </form>

        <p className="text-sm text-muted" style={{ marginTop: '1.25rem' }}>
          Default dev token: <code>dev-secret</code>
        </p>
      </div>
    </div>
  );
}
