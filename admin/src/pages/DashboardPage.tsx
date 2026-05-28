import { useEffect, useState } from 'react';
import { getStats, syncFromFile, syncVersions } from '../lib/api';
import type { Stats, SyncResult, SyncVersionsResult } from '../lib/api';

export default function DashboardPage() {
  const [stats, setStats]                     = useState<Stats | null>(null);
  const [error, setError]                     = useState('');
  const [syncing, setSyncing]                 = useState(false);
  const [syncingVersions, setSyncingVersions] = useState(false);
  const [syncResult, setSyncResult]           = useState<SyncResult | null>(null);
  const [versionResult, setVersionResult]     = useState<SyncVersionsResult | null>(null);

  useEffect(() => {
    getStats().then(setStats).catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed'));
  }, []);

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

  const versionStats = versionResult
    ? versionResult.results.reduce(
        (acc, r) => ({ created: acc.created + r.created, skipped: acc.skipped + r.skipped, errors: acc.errors + (r.error ? 1 : 0) }),
        { created: 0, skipped: 0, errors: 0 },
      )
    : null;

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">Dashboard</h1>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button type="button" className="btn btn--secondary" onClick={() => { void handleSyncVersions(); }} disabled={syncingVersions}>
            {syncingVersions ? 'Syncing…' : '↓ Sync versions (GitHub)'}
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
            GitHub version sync — new versions: {versionStats.created}, already up-to-date: {versionStats.skipped}
            {versionStats.errors > 0 && `, errors: ${versionStats.errors}`}
            {versionResult && versionResult.results.some(r => r.error) && (
              <ul style={{ margin: '0.5rem 0 0', paddingLeft: '1.25rem', fontSize: '0.8rem' }}>
                {versionResult.results.filter(r => r.error).map(r => (
                  <li key={r.plugin}>{r.plugin}: {r.error}</li>
                ))}
              </ul>
            )}
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
      </div>
    </>
  );
}
