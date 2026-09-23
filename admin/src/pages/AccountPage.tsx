import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { deleteAccount, ReauthRequiredError } from '../lib/api';
import { useCurrentUser } from '../hooks/useCurrentUser';
import LegalLinks from '../components/LegalLinks';

export default function AccountPage() {
  const { user } = useCurrentUser();
  const navigate = useNavigate();
  const [confirmation, setConfirmation] = useState('');
  const [deleteOwnedPlugins, setDeleteOwnedPlugins] = useState(false);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [reauthURL, setReauthURL] = useState('');
  const expected = `DELETE ${user?.login ?? ''}`;
  const confirmationMatches = confirmation.trim().toUpperCase() === expected.toUpperCase();

  async function handleDelete(event: React.FormEvent) {
    event.preventDefault();
    setError('');
    setReauthURL('');
    setBusy(true);
    try {
      await deleteAccount({ confirmation, deleteOwnedPlugins, reason });
      navigate('/login', { replace: true });
    } catch (e: unknown) {
      // The server asks for a fresh sign-in rather than a pasted token: the
      // session is an HttpOnly cookie, so there is nothing for the user to
      // copy even if we asked them to.
      if (e instanceof ReauthRequiredError) {
        setReauthURL(`${e.signInURL}`);
        setError(e.message);
      } else {
        setError(e instanceof Error ? e.message : 'Account deletion failed');
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page__body">
      <h1 className="page__title">Account</h1>

      {user && (
        <p className="muted mb-2">
          Signed in as <strong className="text-strong">{user.login}</strong>
          {user.isAdmin ? ' (admin)' : ' (community contributor)'}.
        </p>
      )}

      <div className="alert alert--info">
        Account deletion is a soft-delete operation. Your owned plugins must be explicitly included and all actions are audit logged.
      </div>

      <form onSubmit={(event) => { void handleDelete(event); }} className="form-stack max-w-620">
        <div className="field">
          <label htmlFor="account-confirmation">
            Type <code>{expected}</code> to confirm
          </label>
          <input
            id="account-confirmation"
            className="input"
            value={confirmation}
            onChange={(e) => setConfirmation(e.target.value)}
            aria-describedby="account-confirmation-hint"
            autoComplete="off"
            required
          />
          <span id="account-confirmation-hint" className="field__hint">
            This is case-insensitive but must match exactly otherwise.
          </span>
        </div>

        <label className="checkbox-row" htmlFor="delete-owned-plugins">
          <input
            id="delete-owned-plugins"
            type="checkbox"
            checked={deleteOwnedPlugins}
            onChange={(e) => setDeleteOwnedPlugins(e.target.checked)}
          />
          <span>Also soft-delete my owned plugins and their versions</span>
        </label>

        <div className="field">
          <label htmlFor="account-reason">Reason (optional)</label>
          <textarea
            id="account-reason"
            className="input"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            maxLength={1000}
            aria-describedby="account-reason-hint"
          />
          <span id="account-reason-hint" className="field__hint">
            Recorded in the audit log. Up to 1000 characters.
          </span>
        </div>

        {error && (
          <div className="alert alert--error" role="alert">
            <p className="m-0 text-inherit">{error}</p>
            {reauthURL && (
              <p className="alert-note">
                <a className="btn btn--secondary" href={`${reauthURL}`}>
                  Re-authenticate with GitHub
                </a>
              </p>
            )}
          </div>
        )}

        <p className="field__hint">
          Deleting an account requires a GitHub sign-in from the last five minutes. If it has
          been longer, you will be asked to sign in again before the deletion goes through.
        </p>

        <button
          type="submit"
          className="btn btn--danger"
          disabled={busy || !confirmationMatches}
        >
          {busy ? 'Deleting account…' : 'Delete my account'}
        </button>
      </form>

      <p className="legal-note mt-2">Review the applicable <LegalLinks inline linkClassName="muted" /> before continuing.</p>
    </div>
  );
}
