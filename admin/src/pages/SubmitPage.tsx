import { useState } from 'react';
import { validatePlugin, submitPlugin } from '../lib/api';
import type { ValidationResult } from '../lib/api';

const CATEGORIES = ['analyzer', 'condition', 'generator', 'hook', 'provider', 'updater'];

export default function SubmitPage() {
  const [repoUrl, setRepoUrl] = useState('');
  const [category, setCategory] = useState('');
  const [description, setDescription] = useState('');
  const [license, setLicense] = useState('Apache-2.0');
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
    setValidating(true);
    try {
      const result = await validatePlugin(repoUrl);
      setValidation(result);
      const parsed = parseRepo(repoUrl);
      if (parsed?.name && !category) {
        const m = parsed.name.match(/^(analyzer|condition|generator|hook|provider|updater)-/);
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
      });
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
        <div className="alert" style={{ background: 'var(--success-subtle, #1a3a2a)', borderColor: 'var(--success, #3fb950)', color: 'var(--success, #3fb950)', padding: '1.25rem', borderRadius: 8 }}>
          <strong>Plugin submitted for review!</strong>
          <p style={{ marginTop: '.5rem', marginBottom: 0 }}>
            Your plugin is now <em>pending review</em> by the SemRels maintainers.
            You'll be able to see it in <a href="/plugins" style={{ color: 'inherit' }}>My Plugins</a> with status "pending".
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="page__body page__body--form">
      <h1 style={{ fontSize: 'var(--fs-xl)', marginBottom: '.25rem' }}>Submit a Plugin</h1>
      <p className="muted" style={{ marginBottom: '1.5rem' }}>
        Community plugins must be hosted on GitHub and follow the{' '}
        <a href="https://github.com/SemRels/plugin-template" target="_blank" rel="noreferrer">plugin template</a>.
        After submission, a maintainer will review your plugin before it appears publicly.
      </p>

      {/* Step 1: Validate */}
      <div className="card" style={{ marginBottom: '1rem' }}>
        <h2 style={{ fontSize: 'var(--fs-md)', marginBottom: '.75rem' }}>1. Validate repository</h2>
        <div style={{ display: 'flex', gap: '.5rem' }}>
          <input
            className="input"
            style={{ flex: 1 }}
            placeholder="https://github.com/your-org/analyzer-myanalyzer"
            value={repoUrl}
            onChange={e => { setRepoUrl(e.target.value); setValidation(null); }}
          />
          <button className="btn btn--primary" onClick={() => { void handleValidate(); }} disabled={!repoUrl || validating}>
            {validating ? 'Checking…' : 'Validate'}
          </button>
        </div>

        {validation && (
          <div style={{ marginTop: '1rem' }}>
            <div style={{
              display: 'inline-flex', alignItems: 'center', gap: '.4rem', padding: '.25rem .6rem',
              borderRadius: 4, fontSize: 'var(--fs-sm)', fontWeight: 600, marginBottom: '.75rem',
              background: validation.valid ? 'rgba(63,185,80,.15)' : 'rgba(248,81,73,.15)',
              color: validation.valid ? '#3fb950' : '#f85149',
            }}>
              {validation.valid ? '✓ Passes all checks' : '✗ Some checks failed'}
            </div>
            <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'flex', flexDirection: 'column', gap: '.25rem' }}>
              {validation.checks.map(ch => (
                <li key={ch.id} style={{ display: 'flex', alignItems: 'flex-start', gap: '.4rem', fontSize: 'var(--fs-sm)' }}>
                  <span style={{ color: ch.passed ? '#3fb950' : '#f85149', flexShrink: 0 }}>{ch.passed ? '✓' : '✗'}</span>
                  <span>{ch.label}{ch.message ? ` — ${ch.message}` : ''}</span>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>

      {/* Step 2: Fill details and submit */}
      <form className="card" onSubmit={e => { void handleSubmit(e); }}>
        <h2 style={{ fontSize: 'var(--fs-md)', marginBottom: '.75rem' }}>2. Plugin details</h2>

        <div className="field">
          <label>Description</label>
          <input className="input" placeholder="Short description of what this plugin does"
            value={description} onChange={e => setDescription(e.target.value)} required />
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
          <div className="field">
            <label>Category</label>
            <select className="input" value={category} onChange={e => setCategory(e.target.value)} required>
              <option value="">Select…</option>
              {CATEGORIES.map(c => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          <div className="field">
            <label>License</label>
            <input className="input" value={license} onChange={e => setLicense(e.target.value)} required />
          </div>
        </div>

        {error && <div className="alert alert--error" style={{ marginTop: '.75rem' }}>{error}</div>}

        <button
          type="submit"
          className="btn btn--primary"
          style={{ marginTop: '1rem', width: '100%' }}
          disabled={submitting || !repoUrl || !description || !category}
        >
          {submitting ? 'Submitting…' : 'Submit for review'}
        </button>
      </form>
    </div>
  );
}
