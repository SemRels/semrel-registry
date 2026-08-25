import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { deleteAccount } from '../lib/api';
import { useCurrentUser } from '../hooks/useCurrentUser';
import LegalLinks from '../components/LegalLinks';

export default function AccountPage() {
  const user = useCurrentUser();
  const navigate = useNavigate();
  const [confirmation, setConfirmation] = useState('');
  const [reauthToken, setReauthToken] = useState('');
  const [deleteOwnedPlugins, setDeleteOwnedPlugins] = useState(false);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const expected = `DELETE ${user?.login ?? ''}`;

  async function handleDelete(event: React.FormEvent) {
    event.preventDefault();
    setError('');
    setBusy(true);
    try {
      await deleteAccount({ confirmation, reauthToken, deleteOwnedPlugins, reason });
      localStorage.removeItem('admin_token');
      navigate('/login', { replace: true });
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Account deletion failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page__body">
      <h1 className="page__title">Account</h1>
      <div className="alert alert--info">
        Account deletion is a soft-delete operation. Your owned plugins must be explicitly included and all actions are audit logged.
      </div>
      <form onSubmit={(event) => { void handleDelete(event); }} className="form-stack" style={{ maxWidth: 620 }}>
        <div className="field">
          <label htmlFor="account-confirmation">Type <code>{expected}</code> to confirm</label>
          <input id="account-confirmation" className="input" value={confirmation} onChange={(e) => setConfirmation(e.target.value)} required />
        </div>
        <div className="field">
          <label htmlFor="reauth-token">Re-authenticate with your current sign-in token</label>
          <input id="reauth-token" type="password" className="input" value={reauthToken} onChange={(e) => setReauthToken(e.target.value)} required />
        </div>
        <label className="checkbox-row">
          <input type="checkbox" checked={deleteOwnedPlugins} onChange={(e) => setDeleteOwnedPlugins(e.target.checked)} />
          <span>Also soft-delete my owned plugins and their versions</span>
        </label>
        <div className="field">
          <label htmlFor="account-reason">Reason (optional)</label>
          <textarea id="account-reason" className="input" value={reason} onChange={(e) => setReason(e.target.value)} maxLength={1000} />
        </div>
        {error && <div className="alert alert--error">{error}</div>}
        <button type="submit" className="btn btn--danger" disabled={busy || confirmation.trim().toUpperCase() !== expected || !reauthToken.trim()}>
          {busy ? 'Deleting account…' : 'Delete my account'}
        </button>
      </form>
      <p className="legal-note mt-2">Review the applicable <LegalLinks inline linkClassName="muted" /> before continuing.</p>
    </div>
  );
}
