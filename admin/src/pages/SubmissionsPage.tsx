import { useEffect, useState } from 'react';
import { listPlugins, approvePlugin, rejectPlugin, revalidatePlugin } from '../lib/api';
import type { Plugin, ValidationResult } from '../lib/api';

function CheckIcon({ passed }: { passed: boolean }) {
  return (
    <span style={{ color: passed ? 'var(--success)' : 'var(--danger)', fontWeight: 700, fontSize: '0.8rem', flexShrink: 0 }}>
      {passed ? '✓' : '✗'}
    </span>
  );
}

function ValidationPanel({ checks, summary, valid, validatedAt }: ValidationResult & { validatedAt?: string }) {
  return (
    <div style={{ marginTop: '0.75rem', padding: '0.75rem', background: 'var(--bg)', borderRadius: 6, border: '1px solid var(--border)' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '0.6rem', flexWrap: 'wrap', gap: '0.25rem' }}>
        <div style={{
          display: 'inline-flex', alignItems: 'center', gap: '0.35rem',
          fontSize: 'var(--fs-xs)', fontWeight: 700,
          color: valid ? 'var(--success)' : 'var(--danger)',
        }}>
          {valid ? '✓ All checks passed' : '✗ Some checks failed'}
          {summary && <span style={{ fontWeight: 400, color: 'var(--text-muted)' }}> — {summary}</span>}
        </div>
        {validatedAt && (
          <span style={{ fontSize: 'var(--fs-xs)', color: 'var(--text-muted)' }}>
            checked {new Date(validatedAt).toLocaleString()}
          </span>
        )}
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))', gap: '0.3rem' }}>
        {checks.map(ch => (
          <div key={ch.id} style={{ display: 'flex', alignItems: 'flex-start', gap: '0.4rem', fontSize: 'var(--fs-xs)' }}>
            <CheckIcon passed={ch.passed} />
            <span style={{ color: ch.passed ? 'var(--text)' : 'var(--text-muted)' }}>
              {ch.label}
              {ch.message && <span style={{ color: 'var(--danger)', display: 'block' }}>{ch.message}</span>}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function SubmissionCard({ plugin, onApprove, onReject, onRevalidate }: {
  plugin: Plugin;
  onApprove: () => void;
  onReject: () => void;
  onRevalidate: (result: ValidationResult) => void;
}) {
  const [revalidating, setRevalidating] = useState(false);
  const [revalError, setRevalError]     = useState('');
  const [checks, setChecks]             = useState<ValidationResult | null>(plugin.validationChecks ?? null);
  const [approving, setApproving]       = useState(false);
  const [rejecting, setRejecting]       = useState(false);

  async function handleRevalidate() {
    setRevalidating(true); setRevalError('');
    try {
      const result = await revalidatePlugin(plugin.id);
      setChecks(result);
      onRevalidate(result);
    } catch (e) {
      setRevalError(e instanceof Error ? e.message : 'Revalidation failed');
    } finally { setRevalidating(false); }
  }

  async function handleApprove() {
    setApproving(true);
    try { onApprove(); } finally { setApproving(false); }
  }

  async function handleReject() {
    setRejecting(true);
    try { onReject(); } finally { setRejecting(false); }
  }

  const allPassed = checks?.valid ?? false;

  return (
    <div className="card" style={{ marginBottom: '1rem' }}>
      {/* Header row */}
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: '1rem', flexWrap: 'wrap' }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap', marginBottom: '0.25rem' }}>
            <span style={{ fontWeight: 700, fontSize: '0.95rem' }}>{plugin.name}</span>
            <span className={`badge badge--${plugin.category}`}>{plugin.category}</span>
            {checks && (
              <span style={{
                fontSize: 'var(--fs-xs)', fontWeight: 600, padding: '0.1rem 0.4rem', borderRadius: 4,
                background: allPassed ? 'rgba(63,185,80,.15)' : 'rgba(248,81,73,.15)',
                color: allPassed ? 'var(--success)' : 'var(--danger)',
              }}>
                {allPassed ? '✓ passes standards' : '✗ issues found'}
              </span>
            )}
            {!checks && (
              <span style={{ fontSize: 'var(--fs-xs)', color: 'var(--text-muted)' }}>not yet validated</span>
            )}
          </div>
          <div style={{ fontSize: 'var(--fs-sm)', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
            {plugin.description}
          </div>
          <div style={{ fontSize: 'var(--fs-xs)', color: 'var(--text-muted)', display: 'flex', gap: '1rem', flexWrap: 'wrap' }}>
            <span>👤 {plugin.author}</span>
            <span>📄 {plugin.license}</span>
            <a href={plugin.repository} target="_blank" rel="noreferrer" style={{ color: 'var(--accent)' }}>
              ↗ {plugin.repository.replace('https://github.com/', '')}
            </a>
            <span>🕐 {new Date(plugin.createdAt).toLocaleDateString()}</span>
          </div>
        </div>

        {/* Action buttons */}
        <div style={{ display: 'flex', gap: '0.4rem', flexShrink: 0, alignItems: 'flex-start' }}>
          <button
            className="btn btn--sm"
            title="Re-run validation checks"
            onClick={() => { void handleRevalidate(); }}
            disabled={revalidating}
            style={{ fontSize: 'var(--fs-xs)' }}
          >
            {revalidating ? '⏳' : checks ? '↻ Re-check' : '▶ Run check'}
          </button>
          <button
            className="btn btn--primary btn--sm"
            onClick={() => { void handleApprove(); }}
            disabled={approving}
          >
            {approving ? '…' : '✓ Approve'}
          </button>
          <button
            className="btn btn--danger btn--sm"
            onClick={() => { void handleReject(); }}
            disabled={rejecting}
          >
            {rejecting ? '…' : '✗ Reject'}
          </button>
        </div>
      </div>

      {revalError && <div className="alert alert--error" style={{ marginTop: '0.5rem', padding: '0.3rem 0.5rem' }}>{revalError}</div>}

      {/* Validation panel — shows stored results from DB automatically */}
      {checks && <ValidationPanel {...checks} validatedAt={plugin.validatedAt} />}
    </div>
  );
}

const PAGE_SIZE = 20;

export default function SubmissionsPage() {
  const [plugins, setPlugins]   = useState<Plugin[]>([]);
  const [loading, setLoading]   = useState(true);
  const [error, setError]       = useState('');
  const [filter, setFilter]     = useState<'pending' | 'rejected' | 'all'>('pending');
  const [page, setPage]         = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [total, setTotal]       = useState(0);

  useEffect(() => { setPage(1); }, [filter]);
  useEffect(() => { void load(); }, [filter, page]); // eslint-disable-line react-hooks/exhaustive-deps

  async function load() {
    setLoading(true); setError('');
    try {
      const status = filter === 'all' ? undefined : filter;
      const r = await listPlugins({ status, limit: PAGE_SIZE, page });
      setPlugins(r.data);
      setTotalPages(r.pagination?.pages ?? 1);
      setTotal(r.pagination?.total ?? r.data.length);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load');
    } finally { setLoading(false); }
  }

  async function handleApprove(id: number) {
    try {
      await approvePlugin(id);
      setPlugins(prev => prev.filter(p => p.id !== id));
      setTotal(t => Math.max(0, t - 1));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Approve failed');
    }
  }

  async function handleReject(id: number) {
    try {
      await rejectPlugin(id);
      setPlugins(prev => prev.map(p => p.id === id ? { ...p, status: 'rejected' } : p));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Reject failed');
    }
  }

  function handleRevalidated(id: number, result: ValidationResult) {
    setPlugins(prev => prev.map(p => p.id === id ? { ...p, validationChecks: result } : p));
  }

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">
          Submissions
          {total > 0 && filter !== 'all' && (
            <span style={{ background: 'var(--danger)', color: '#fff', borderRadius: 10, padding: '0 6px', fontSize: 'var(--fs-xs)', marginLeft: '0.5rem', verticalAlign: 'middle' }}>
              {total}
            </span>
          )}
        </h1>
        <div style={{ display: 'flex', gap: '0.4rem' }}>
          {(['pending', 'rejected', 'all'] as const).map(f => (
            <button key={f} className={`btn btn--sm${filter === f ? ' btn--primary' : ''}`} onClick={() => setFilter(f)}>
              {f === 'all' ? 'All' : f.charAt(0).toUpperCase() + f.slice(1)}
            </button>
          ))}
        </div>
      </div>

      <div className="page__body">
        {error && <div className="alert alert--error">{error}</div>}
        {loading && <p className="muted">Loading…</p>}
        {!loading && plugins.length === 0 && (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '3rem 0', gap: '.75rem', textAlign: 'center' }}>
            <span style={{ fontSize: '3.5rem', lineHeight: 1 }}>
              {filter === 'pending' ? '🎉' : '📭'}
            </span>
            <p className="muted" style={{ margin: 0, fontSize: 'var(--fs-md)' }}>
              {filter === 'pending' ? 'No pending submissions.' : `No ${filter} submissions.`}
            </p>
          </div>
        )}
        {!loading && plugins.map(p => (
          <SubmissionCard
            key={p.id}
            plugin={p}
            onApprove={() => { void handleApprove(p.id); }}
            onReject={() => { void handleReject(p.id); }}
            onRevalidate={(result) => handleRevalidated(p.id, result)}
          />
        ))}

        {/* Pagination */}
        {!loading && totalPages > 1 && (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem', marginTop: '1rem' }}>
            <button className="btn btn--secondary btn--sm" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>← Prev</button>
            <span className="muted" style={{ fontSize: 'var(--fs-sm)' }}>Page {page} / {totalPages}</span>
            <button className="btn btn--secondary btn--sm" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next →</button>
          </div>
        )}
      </div>
    </>
  );
}
