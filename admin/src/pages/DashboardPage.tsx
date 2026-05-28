import { useEffect, useState } from 'react';
import { getStats, syncFromFile, syncVersions, syncGitHubOrg, listPlugins, approvePlugin, rejectPlugin } from '../lib/api';
import type { Stats, SyncResult, SyncVersionsResult, OrgSyncResult, Plugin } from '../lib/api';

export default function DashboardPage() {
  const [stats, setStats]                     = useState<Stats | null>(null);
  const [error, setError]                     = useState('');
  const [syncing, setSyncing]                 = useState(false);
  const [syncingVersions, setSyncingVersions] = useState(false);
  const [syncingOrg, setSyncingOrg]           = useState(false);
  const [syncResult, setSyncResult]           = useState<SyncResult | null>(null);
  const [versionResult, setVersionResult]     = useState<SyncVersionsResult | null>(null);
  const [orgResult, setOrgResult]             = useState<OrgSyncResult | null>(null);
  const [pending, setPending]                 = useState<Plugin[]>([]);
  const [pendingLoading, setPendingLoading]   = useState(true);

  useEffect(() => {
    getStats().then(setStats).catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed'));
    loadPending();
  }, []);

  async function loadPending() {
    setPendingLoading(true);
    try {
      const r = await listPlugins({ status: 'pending', limit: 50 });
      setPending(r.data);
    } catch { setPending([]); }
    finally { setPendingLoading(false); }
  }

  async function handleSync() {
    setSyncing(true); setSyncResult(null); setError('');
    try {
      const r = await syncFromFile();
      setSyncResult(r);
      setStats(await getStats());
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Sync failed');
    } finally { setSyncing(false); }
  }

  async function handleSyncVersions() {
    setSyncingVersions(true); setVersionResult(null); setError('');
    try {
      const r = await syncVersions();
      setVersionResult(r);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Version sync failed');
    } finally { setSyncingVersions(false); }
  }

  async function handleSyncOrg() {
    setSyncingOrg(true); setOrgResult(null); setError('');
    try {
      const r = await syncGitHubOrg();
      setOrgResult(r);
      setStats(await getStats());
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Org sync failed');
    } finally { setSyncingOrg(false); }
  }

  async function handleApprove(id: number) {
    try {
      await approvePlugin(id);
      await loadPending();
      setStats(await getStats());
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Approve failed');
    }
  }

  async function handleReject(id: number) {
    try {
      await rejectPlugin(id);
      await loadPending();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Reject failed');
    }
  }

  const versionStats = versionResult
    ? versionResult.results.reduce(
        (acc, r) => ({ created: acc.created + r.created, skipped: acc.skipped + r.skipped, errors: acc.errors + (r.error ? 1 : 0) }),
        { created: 0, skipped: 0, errors: 0 },
      )
    : null;

  const orgStats = orgResult
    ? orgResult.results.reduce(
        (acc, r) => ({ created: acc.created + (r.action === 'created' ? 1 : 0), updated: acc.updated + (r.action === 'updated' ? 1 : 0), errors: acc.errors + (r.error ? 1 : 0) }),
        { created: 0, updated: 0, errors: 0 },
      )
    : null;

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">Dashboard</h1>
        <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
          <button type="button" className="btn btn--secondary" onClick={() => { void handleSyncVersions(); }} disabled={syncingVersions}>
            {syncingVersions ? 'Syncing…' : '↓ Sync versions'}
          </button>
          <button type="button" className="btn btn--secondary" onClick={() => { void handleSyncOrg(); }} disabled={syncingOrg}>
            {syncingOrg ? 'Scanning…' : '⟳ Sync GitHub org'}
          </button>
          <button type="button" className="btn btn--primary" onClick={() => { void handleSync(); }} disabled={syncing}>
            {syncing ? 'Syncing…' : '⟳ Sync plugins.json'}
          </button>
        </div>
      </div>
      <div className="page__body">
        {error && <div className="alert alert--error">{error}</div>}
        {syncResult && (
          <div className="alert alert--success">
            plugins.json sync — created: {syncResult.created}, updated: {syncResult.updated}, failed: {syncResult.failed}
          </div>
        )}
        {versionStats && (
          <div className={versionStats.errors > 0 ? 'alert alert--error' : 'alert alert--success'}>
            Version sync — new: {versionStats.created}, up-to-date: {versionStats.skipped}
            {versionStats.errors > 0 && `, errors: ${versionStats.errors}`}
          </div>
        )}
        {orgStats && (
          <div className={orgStats.errors > 0 ? 'alert alert--error' : 'alert alert--success'}>
            GitHub org sync — discovered: {orgResult!.total}, new: {orgStats.created}, updated: {orgStats.updated}
            {orgStats.errors > 0 && `, errors: ${orgStats.errors}`}
          </div>
        )}

        {!stats && !error && <p className="muted">Loading…</p>}
        {stats && (
          <div className="stat-grid">
            <div className="stat-card">
              <div className="stat-card__label">Total</div>
              <div className="stat-card__value">{stats.totalPlugins}</div>
            </div>
            {Object.entries(stats.categories).map(([cat, count]) => (
              <div key={cat} className="stat-card">
                <div className="stat-card__label">{cat}</div>
                <div className="stat-card__value">{count}</div>
              </div>
            ))}
          </div>
        )}

        {/* Pending submissions */}
        <div style={{ marginTop: '2rem' }}>
          <h2 style={{ fontSize: 'var(--fs-md)', marginBottom: '1rem' }}>
            Pending submissions {pending.length > 0 && <span style={{ background: '#f85149', color: '#fff', borderRadius: 10, padding: '0 6px', fontSize: 'var(--fs-xs)', marginLeft: '.4rem' }}>{pending.length}</span>}
          </h2>
          {pendingLoading && <p className="muted">Loading…</p>}
          {!pendingLoading && pending.length === 0 && <p className="muted">No pending submissions. 🎉</p>}
          {pending.map(p => (
            <div key={p.id} className="table__row" style={{ display: 'flex', alignItems: 'center', gap: '.75rem', padding: '.6rem 0', borderBottom: '1px solid var(--border)' }}>
              <div style={{ flex: 1 }}>
                <a href={p.repository} target="_blank" rel="noreferrer" style={{ fontWeight: 600 }}>{p.name}</a>
                <span className="muted" style={{ fontSize: 'var(--fs-xs)', marginLeft: '.5rem' }}>by {p.author}</span>
                <div className="muted" style={{ fontSize: 'var(--fs-xs)' }}>{p.description}</div>
              </div>
              <div style={{ display: 'flex', gap: '.4rem' }}>
                <button className="btn btn--primary" style={{ padding: '4px 12px', fontSize: 'var(--fs-sm)' }} onClick={() => { void handleApprove(p.id); }}>Approve</button>
                <button className="btn btn--danger" style={{ padding: '4px 12px', fontSize: 'var(--fs-sm)' }} onClick={() => { void handleReject(p.id); }}>Reject</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}
