import { useEffect, useState } from 'react';
import { getStats, syncFromFile } from '../lib/api';
import type { Stats, SyncResult } from '../lib/api';

const CATEGORY_ICONS: Record<string, string> = {
  provider: '🔗',
  analyzer: '🔍',
  condition: '⚡',
  hook: '🪝',
  updater: '⬆️',
};

export default function DashboardPage() {
  const [stats, setStats] = useState<Stats | null>(null);
  const [error, setError] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [syncResult, setSyncResult] = useState<SyncResult | null>(null);

  useEffect(() => {
    getStats()
      .then(setStats)
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed to load stats'));
  }, []);

  async function handleSync() {
    setSyncing(true);
    setSyncResult(null);
    try {
      const result = await syncFromFile();
      setSyncResult(result);
      const updated = await getStats();
      setStats(updated);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Sync failed');
    } finally {
      setSyncing(false);
    }
  }

  return (
    <>
      <div className="flex-between" style={{ marginBottom: '0.25rem' }}>
        <h1 className="page-title">Dashboard</h1>
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => { void handleSync(); }}
          disabled={syncing}
        >
          {syncing ? <><span className="spinner" /> Syncing…</> : '⟳ Sync from plugins.json'}
        </button>
      </div>
      <p className="page-subtitle">Registry statistics and quick actions.</p>

      {error && <div className="alert alert-error">{error}</div>}
      {syncResult && (
        <div className="alert alert-success">
          Sync complete — created: {syncResult.created}, updated: {syncResult.updated}, failed: {syncResult.failed}
        </div>
      )}

      {!stats && !error && (
        <div className="loading-block">
          <span className="spinner" /> Loading stats…
        </div>
      )}

      {stats && (
        <>
          <div className="grid-4 mt-2">
            <div className="metric-card">
              <div className="metric-label">Total Plugins</div>
              <div className="metric-value">{stats.totalPlugins}</div>
            </div>
            {Object.entries(stats.categories).map(([cat, count]) => (
              <div key={cat} className="metric-card">
                <div className="metric-label">{CATEGORY_ICONS[cat] ?? '📦'} {cat}</div>
                <div className="metric-value">{count}</div>
              </div>
            ))}
          </div>

          <div className="card mt-3">
            <h2 style={{ margin: '0 0 0.75rem', fontSize: '1rem' }}>Quick Actions</h2>
            <div className="flex-row">
              <a href="/plugins" className="btn btn-ghost">🔌 Manage Plugins</a>
              <a href="http://localhost:3000" target="_blank" rel="noopener" className="btn btn-ghost">
                ↗ Open Registry Web UI
              </a>
              <a href="http://localhost:8080/api/v1/plugins" target="_blank" rel="noopener" className="btn btn-ghost">
                ↗ Raw API
              </a>
            </div>
          </div>
        </>
      )}
    </>
  );
}
