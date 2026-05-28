import { useEffect, useState } from 'react';
import { getStats, syncFromFile } from '../lib/api';
import type { Stats, SyncResult } from '../lib/api';

export default function DashboardPage() {
  const [stats, setStats]       = useState<Stats | null>(null);
  const [error, setError]       = useState('');
  const [syncing, setSyncing]   = useState(false);
  const [syncResult, setSyncResult] = useState<SyncResult | null>(null);

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

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">Dashboard</h1>
        <button type="button" className="btn btn--primary" onClick={() => { void handleSync(); }} disabled={syncing}>
          {syncing ? 'Syncing…' : '⟳ Sync plugins.json'}
        </button>
      </div>
      <div className="page__body">
        {error      && <div className="alert alert--error">{error}</div>}
        {syncResult && <div className="alert alert--success">Sync done — created: {syncResult.created}, updated: {syncResult.updated}, failed: {syncResult.failed}</div>}

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
