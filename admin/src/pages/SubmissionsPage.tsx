import { useEffect, useState } from 'react';
import { listPlugins, approvePlugin, rejectPlugin, revalidatePlugin } from '../lib/api';
import type { Plugin, ValidationResult } from '../lib/api';
import ReasonDialog from '../components/ReasonDialog';
import { TableSkeleton, EmptyState } from '../components/LoadingState';

function CheckIcon({ passed }: { passed: boolean }) {
  return (
    <span className={passed ? 'validation-panel__icon--pass' : 'validation-panel__icon--fail'}>
      {passed ? '✓' : '✗'}
    </span>
  );
}

function ValidationPanel({ checks, summary, valid, validatedAt }: ValidationResult & { validatedAt?: string }) {
  return (
    <div className="validation-panel">
      <div className="flex items-center justify-between mb-1 flex-wrap gap-xs">
        <div className={`inline-flex items-center gap-xs text-xs font-bold ${valid ? 'validation-panel__status--pass' : 'validation-panel__status--fail'}`}>
          {valid ? '✓ All checks passed' : '✗ Some checks failed'}
          {summary && <span className="validation-panel__summary"> — {summary}</span>}
        </div>
        {validatedAt && (
          <span className="text-xs muted">
            checked {new Date(validatedAt).toLocaleString()}
          </span>
        )}
      </div>
      <div className="validation-panel__grid">
        {checks.map(ch => (
          <div key={ch.id} className="validation-panel__check">
            <CheckIcon passed={ch.passed} />
            <span className={ch.passed ? '' : 'muted'}>
              {ch.label}
              {ch.message && <span className="validation-panel__message">{ch.message}</span>}
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
    <div className="card mb-2">
      {/* Header row */}
      <div className="flex items-start gap-md flex-wrap">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-sm flex-wrap mb-1">
            <span className="submission-card__name">{plugin.name}</span>
            <span className={`badge badge--${plugin.category}`}>{plugin.category}</span>
            {checks && (
              <span className={`submission-status ${allPassed ? 'submission-status--pass' : 'submission-status--fail'}`}>
                {allPassed ? '✓ passes standards' : '✗ issues found'}
              </span>
            )}
            {!checks && (
              <span className="text-xs muted">not yet validated</span>
            )}
          </div>
          <div className="text-sm muted mb-1">
            {plugin.description}
          </div>
          <div className="text-xs muted flex gap-md flex-wrap">
            <span>👤 {plugin.author}</span>
            <span>📄 {plugin.license}</span>
            <a href={plugin.repository} target="_blank" rel="noreferrer" className="link-accent">
              ↗ {plugin.repository.replace('https://github.com/', '')}
            </a>
            <span>🕐 {new Date(plugin.createdAt).toLocaleDateString()}</span>
          </div>
        </div>

        {/* Action buttons */}
        <div className="flex gap-xs no-shrink items-start">
          <button
            className="btn btn--sm text-xs"
            title="Re-run validation checks"
            onClick={() => { void handleRevalidate(); }}
            disabled={revalidating}
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

      {revalError && <div className="alert alert--error mt-1">{revalError}</div>}

      {/* A rejected submission carries the reviewer's explanation, so a
          maintainer revisiting the list can see why it was turned down. */}
      {plugin.status === 'rejected' && plugin.rejectionReason && (
        <div className="alert alert--error mt-1">
          <strong>Rejected{plugin.reviewedBy ? ` by ${plugin.reviewedBy}` : ''}:</strong>{' '}
          {plugin.rejectionReason}
        </div>
      )}

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
  // The submission being rejected, held while its reason is collected.
  const [rejecting, setRejecting] = useState<Plugin | null>(null);
  const [rejectBusy, setRejectBusy] = useState(false);
  const [rejectError, setRejectError] = useState('');

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

  async function handleReject(id: number, reason: string) {
    setRejectBusy(true);
    setRejectError('');
    try {
      const updated = await rejectPlugin(id, reason);
      setPlugins(prev => prev.map(p => p.id === id ? { ...p, status: 'rejected', rejectionReason: updated.rejectionReason } : p));
      setRejecting(null);
    } catch (e) {
      setRejectError(e instanceof Error ? e.message : 'Reject failed');
    } finally {
      setRejectBusy(false);
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
            <span className="count-badge count-badge--middle">
              {total}
            </span>
          )}
        </h1>
        <div className="flex gap-xs">
          {(['pending', 'rejected', 'all'] as const).map(f => (
            <button key={f} className={`btn btn--sm${filter === f ? ' btn--primary' : ''}`} onClick={() => setFilter(f)}>
              {f === 'all' ? 'All' : f.charAt(0).toUpperCase() + f.slice(1)}
            </button>
          ))}
        </div>
      </div>

      <div className="page__body">
        {error && <div className="alert alert--error">{error}</div>}
        {loading && <TableSkeleton label="Loading submissions" count={3} />}
        {!loading && plugins.length === 0 && (
          <EmptyState
            title={filter === 'pending' ? 'Nothing waiting for review' : `No ${filter} submissions`}
          >
            {filter === 'pending'
              ? 'Community submissions land here. Approving one publishes it to the catalogue; rejecting one asks for a reason the author will see.'
              : 'Nothing matches this filter. Switch to another to see the rest.'}
          </EmptyState>
        )}
        {!loading && plugins.map(p => (
          <SubmissionCard
            key={p.id}
            plugin={p}
            onApprove={() => { void handleApprove(p.id); }}
            onReject={() => { setRejectError(''); setRejecting(p); }}
            onRevalidate={(result) => handleRevalidated(p.id, result)}
          />
        ))}

        {/* Rejecting asks for a reason first: the author sees it on their
            plugin list, and "rejected" alone tells them nothing to fix. */}
        <ReasonDialog
          open={rejecting !== null}
          title={`Reject ${rejecting?.name ?? ''}?`}
          message="The submission stays visible to its author with this explanation attached. They can address it and submit again."
          label="Reason for rejection"
          placeholder="e.g. The repository has no release workflow, so the registry cannot discover versions."
          confirmLabel="Reject submission"
          busyLabel="Rejecting…"
          busy={rejectBusy}
          error={rejectError}
          onClose={() => { if (!rejectBusy) setRejecting(null); }}
          onConfirm={(reason) => { if (rejecting) void handleReject(rejecting.id, reason); }}
        />

        {/* Pagination */}
        {!loading && totalPages > 1 && (
          <div className="flex items-center justify-center gap-sm mt-2">
            <button className="btn btn--secondary btn--sm" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>← Prev</button>
            <span className="muted text-sm">Page {page} / {totalPages}</span>
            <button className="btn btn--secondary btn--sm" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next →</button>
          </div>
        )}
      </div>
    </>
  );
}
