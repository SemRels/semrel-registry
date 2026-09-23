import { useState } from 'react';
import { Link } from 'react-router-dom';
import { validatePlugin, submitPlugin, verifyRepositoryOwnership } from '../lib/api';
import type { ValidationResult, OwnershipResult } from '../lib/api';
import LegalLinks from '../components/LegalLinks';
import StatusIcon from '../components/StatusIcon';

const CATEGORIES = ['analyzer', 'condition', 'generator', 'hook', 'provider', 'updater', 'packager', 'publisher'];

export default function SubmitPage() {
  const [repoUrl, setRepoUrl] = useState('');
  const [category, setCategory] = useState('');
  const [description, setDescription] = useState('');
  const [license, setLicense] = useState('Apache-2.0');
  const [notifyEmail, setNotifyEmail] = useState('');
  const [ownership, setOwnership] = useState<OwnershipResult | null>(null);
  const [validation, setValidation] = useState<ValidationResult | null>(null);
  const [validating, setValidating] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [error, setError] = useState('');

  function parseRepo(url: string) {
    const m = url.match(/github\.com\/([^/]+)\/([^/]+)/);
    return m ? { owner: m[1], name: m[2].replace(/\.git$/, '') } : null;
  }

  async function handleValidate() {
    setError('');
    setValidation(null);
    setOwnership(null);
    setValidating(true);
    try {
      // Ownership is checked here rather than at submit time: a claim that
      // cannot be made is something the submitter must go and fix in their
      // repository, which is a poor thing to discover after filling in a form.
      const [result, owner] = await Promise.all([
        validatePlugin(repoUrl),
        verifyRepositoryOwnership(repoUrl).catch(() => null),
      ]);
      setValidation(result);
      setOwnership(owner);

      const parsed = parseRepo(repoUrl);
      if (parsed?.name && !category) {
        const m = parsed.name.match(/^(analyzer|condition|generator|hook|provider|updater|packager|publisher)-/);
        if (m) setCategory(m[1]);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Validation failed');
    } finally {
      setValidating(false);
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      const parsed = parseRepo(repoUrl);
      if (!parsed) throw new Error('Invalid GitHub repository URL');
      await submitPlugin({
        name: parsed.name,
        description,
        category,
        repository: repoUrl.replace(/\.git$/, ''),
        license,
        tags: [category],
      }, notifyEmail);
      setSubmitted(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Submission failed');
    } finally {
      setSubmitting(false);
    }
  }

  if (submitted) {
    return (
      <div className="page__body page__body--form">
        {/* role="status": submission succeeds without a navigation, so this is
            the only signal that anything happened. */}
        <div
          className="alert alert--success alert--panel"
          role="status"
        >
          <strong>Plugin submitted for review</strong>
          <p className="mt-1 m-0 text-inherit">
            Your plugin is now <em>pending review</em> by the SemRels maintainers. It appears
            in <Link to="/admin/plugins" className="link-inherit">My Plugins</Link> with
            status &ldquo;pending&rdquo; until a maintainer approves or rejects it. If it is
            rejected you will see the reason there.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="page__body page__body--form">
      <h1 className="page__title mb-1">Submit a Plugin</h1>
      <p className="muted mb-3">
        Community plugins must be hosted on GitHub and follow the{' '}
        <a href="https://github.com/SemRels/plugin-template" target="_blank" rel="noreferrer">plugin template</a>.
        After submission, a maintainer will review your plugin before it appears publicly.
      </p>

      {/* Step 1: Validate */}
      <div className="card mb-2">
        <h2 className="section-title">1. Validate repository</h2>
        <div className="field">
          <label htmlFor="repo-url">Repository URL</label>
          <div className="flex gap-sm">
            <input
              id="repo-url"
              className="input flex-1"
              type="url"
              placeholder="https://github.com/your-org/analyzer-myanalyzer"
              value={repoUrl}
              aria-describedby="repo-url-hint"
              onChange={e => { setRepoUrl(e.target.value); setValidation(null); }}
            />
            <button type="button" className="btn btn--primary" onClick={() => { void handleValidate(); }} disabled={!repoUrl || validating}>
              {validating ? 'Checking…' : 'Validate'}
            </button>
          </div>
          <span id="repo-url-hint" className="field__hint">
            A public GitHub repository named <code>&lt;category&gt;-&lt;name&gt;</code>, for example <code>analyzer-myanalyzer</code>.
          </span>
        </div>

        {/* aria-live: the result arrives asynchronously, so without a live
            region a screen-reader user is given no indication it appeared. */}
        <div aria-live="polite" aria-busy={validating}>
          {validating && <p className="muted">Checking the repository against the plugin standards…</p>}

          {/* Ownership is the gate: the standards checks can all pass on a
              repository the submitter has nothing to do with. */}
          {ownership && (
            <div
              className={`${ownership.verified ? 'alert alert--info' : 'alert alert--error'} mt-2`}
              role={ownership.verified ? undefined : 'alert'}
            >
              {ownership.verified ? (
                <span>
                  <strong>Repository ownership confirmed</strong>
                  {ownership.method === 'claim-file' && ' via the claim file'}
                  {ownership.method === 'public-org-member' && ' — you are a public member of this organisation'}
                  {ownership.method === 'account-owner' && ' — the repository is on your account'}
                  .
                </span>
              ) : (
                <>
                  <strong>You have not shown that you control this repository.</strong>
                  {ownership.issue && <p className="alert-note">{ownership.issue}</p>}
                  {ownership.howToFix && <p className="alert-note">{ownership.howToFix}</p>}
                </>
              )}
            </div>
          )}

          {validation && (
            <div className="mt-2">
              <div className={`validation-chip ${validation.valid ? 'validation-chip--pass' : 'validation-chip--fail'}`}>
                <span aria-hidden="true">{validation.valid ? '✓' : '✗'}</span>
                {validation.valid ? 'Passes all checks' : 'Some checks failed'}
              </div>
              <ul className="checklist">
                {validation.checks.map(ch => (
                  <li key={ch.id} className="checklist__item">
                    <StatusIcon passed={ch.passed} />
                    <span>{ch.label}{ch.message ? ` — ${ch.message}` : ''}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>

      {/* Step 2: Fill details and submit */}
      <form className="card" onSubmit={e => { void handleSubmit(e); }}>
        <h2 className="section-title">2. Plugin details</h2>

        <div className="field">
          <label htmlFor="plugin-description">Description</label>
          <input id="plugin-description" className="input" placeholder="Short description of what this plugin does"
            value={description} onChange={e => setDescription(e.target.value)} required />
        </div>

        <div className="form-grid-2">
          <div className="field">
            <label htmlFor="plugin-category">Category</label>
            <select id="plugin-category" className="input" value={category} onChange={e => setCategory(e.target.value)} required>
              <option value="">Select…</option>
              {CATEGORIES.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          <div className="field">
            <label htmlFor="plugin-license">License</label>
            <input id="plugin-license" className="input" value={license} onChange={e => setLicense(e.target.value)} required />
          </div>
        </div>

        {/* Optional and opt-in. Without it the outcome is still recorded and
            shown on the submitter's plugin list — they just have to come back
            and look, which is what the review loop used to require. */}
        <div className="field">
          <label htmlFor="notify-email">Email for the review result <span className="muted">(optional)</span></label>
          <input
            id="notify-email"
            className="input"
            type="email"
            autoComplete="email"
            placeholder="you@example.com"
            value={notifyEmail}
            onChange={e => setNotifyEmail(e.target.value)}
            aria-describedby="notify-email-hint"
          />
          <span id="notify-email-hint" className="field__hint">
            Used once, to tell you whether this plugin was accepted, and for
            nothing else. It is never shown publicly and is deleted with your
            account. Leave it empty to check back here instead.
          </span>
        </div>

        {error && <div className="alert alert--error mt-md" role="alert">{error}</div>}

        <button
          type="submit"
          className="btn btn--primary mt-2 w-full"
          disabled={submitting || !repoUrl || !description || !category || ownership?.verified !== true}
        >
          {submitting ? 'Submitting…' : 'Submit for review'}
        </button>
        {ownership?.verified !== true && (
          <p className="field__hint mt-1">
            Validate the repository above first — submitting requires showing
            that you control it.
          </p>
        )}
        <p className="legal-note mt-1">
          By submitting, you confirm that you are authorised to publish this metadata and agree to the semrel contributor terms.
        </p>
        <LegalLinks inline className="legal-note__links" linkClassName="muted" />
      </form>
    </div>
  );
}
