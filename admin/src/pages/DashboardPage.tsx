import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getStats, syncFromFile, syncVersions, syncGitHubOrg, listPlugins } from '../lib/api';
import type { Stats, SyncResult, SyncVersionsResult, OrgSyncResult, Plugin } from '../lib/api';
import { useCurrentUser } from '../hooks/useCurrentUser';

export default function DashboardPage() {
  const user    = useCurrentUser();
  const navigate = useNavigate();
  const isAdmin = user?.isAdmin === true;
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
  const [seriesRange, setSeriesRange]         = useState<'day' | 'week' | 'month'>('day');

  useEffect(() => {
    getStats().then(setStats).catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed'));
    if (isAdmin) loadPending();
  }, [isAdmin]);

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

  const activeSeries = stats?.series?.[seriesRange] ?? [];
  const maxSeriesValue = activeSeries.reduce((max, point) => {
    const localMax = Math.max(point.views ?? 0, point.downloads ?? 0);
    return Math.max(localMax, max);
  }, 1);

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
          <>
          <div className="stat-grid">
            <div className="stat-card">
              <div className="stat-card__label">Total</div>
              <div className="stat-card__value">{stats.totalPlugins}</div>
            </div>
            <div className="stat-card">
              <div className="stat-card__label">Views</div>
              <div className="stat-card__value">{stats.totalViews.toLocaleString()}</div>
            </div>
            <div className="stat-card">
              <div className="stat-card__label">Downloads</div>
              <div className="stat-card__value">{stats.totalDownloads.toLocaleString()}</div>
            </div>
            {Object.entries(stats.categories).map(([cat, count]) => (
              <div key={cat} className="stat-card">
                <div className="stat-card__label">{cat}</div>
                <div className="stat-card__value">{count}</div>
              </div>
            ))}
          </div>

          <div className="card" style={{ marginTop: '1rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '1rem', flexWrap: 'wrap' }}>
              <h2 style={{ margin: 0, fontSize: 'var(--fs-md)' }}>Traffic trend</h2>
              <div style={{ display: 'flex', gap: '.5rem' }}>
                {(['day', 'week', 'month'] as const).map((range) => (
                  <button
                    key={range}
                    type="button"
                    className={`btn btn--sm ${seriesRange === range ? 'btn--primary' : ''}`}
                    onClick={() => setSeriesRange(range)}
                  >
                    {range}
                  </button>
                ))}
              </div>
            </div>
            {activeSeries.length === 0 ? (
              <p className="muted" style={{ marginTop: '.75rem' }}>No trend data yet.</p>
            ) : (
              <div style={{ marginTop: '.75rem', display: 'grid', gap: '.5rem' }}>
                {activeSeries.map((point) => {
                  const downloadWidth = Math.max(2, Math.round(((point.downloads ?? 0) / maxSeriesValue) * 100));
                  const viewWidth = Math.max(2, Math.round(((point.views ?? 0) / maxSeriesValue) * 100));
                  return (
                    <div key={point.period} style={{ display: 'grid', gap: '.25rem' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 'var(--fs-xs)' }}>
                        <span>{point.period}</span>
                        <span className="muted">V {point.views.toLocaleString()} · D {point.downloads.toLocaleString()}</span>
                      </div>
                      <div style={{ display: 'grid', gap: '.25rem' }}>
                        <div style={{ width: `${viewWidth}%`, height: 6, borderRadius: 999, background: 'rgba(56,139,253,.6)' }} />
                        <div style={{ width: `${downloadWidth}%`, height: 6, borderRadius: 999, background: 'rgba(63,185,80,.75)' }} />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          <div style={{ marginTop: '1rem', display: 'grid', gap: '1rem' }}>
            <div className="card">
              <h2 style={{ margin: 0, fontSize: 'var(--fs-md)', marginBottom: '.75rem' }}>Top plugins</h2>
              {!stats.topPlugins || stats.topPlugins.length === 0 ? (
                <p className="muted">No plugin metrics yet.</p>
              ) : (
                <div className="table-wrap">
                  <table className="table--stack">
                    <thead><tr><th>Plugin</th><th>Category</th><th>Views</th><th>Downloads</th></tr></thead>
                    <tbody>
                      {stats.topPlugins.map((item) => (
                        <tr key={item.pluginId}>
                          <td data-label="Plugin">{item.namespace ? `${item.namespace}/` : ''}{item.name}</td>
                          <td data-label="Category" className="muted">{item.category}</td>
                          <td data-label="Views">{item.views.toLocaleString()}</td>
                          <td data-label="Downloads">{item.downloads.toLocaleString()}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>

            <div className="card">
              <h2 style={{ margin: 0, fontSize: 'var(--fs-md)', marginBottom: '.75rem' }}>Top versions</h2>
              {!stats.topVersions || stats.topVersions.length === 0 ? (
                <p className="muted">No version metrics yet.</p>
              ) : (
                <div className="table-wrap">
                  <table className="table--stack">
                    <thead><tr><th>Version</th><th>Plugin</th><th>Views</th><th>Downloads</th></tr></thead>
                    <tbody>
                      {stats.topVersions.map((item) => (
                        <tr key={item.versionId}>
                          <td data-label="Version"><code>v{item.version}</code></td>
                          <td data-label="Plugin">{item.namespace ? `${item.namespace}/` : ''}{item.pluginName}</td>
                          <td data-label="Views">{item.views.toLocaleString()}</td>
                          <td data-label="Downloads">{item.downloads.toLocaleString()}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </div>
          </>
        )}

        {/* Pending submissions — admin only */}
        {isAdmin && (
          <div style={{ marginTop: '2rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem' }}>
              <h2 style={{ fontSize: 'var(--fs-md)', margin: 0 }}>
                Pending submissions
                {pending.length > 0 && (
                  <span style={{ background: '#f85149', color: '#fff', borderRadius: 10, padding: '0 6px', fontSize: 'var(--fs-xs)', marginLeft: '.4rem' }}>
                    {pending.length}
                  </span>
                )}
              </h2>
              <button
                className="btn btn--primary"
                style={{ padding: '4px 14px', fontSize: 'var(--fs-sm)' }}
                onClick={() => navigate('/admin/submissions')}
              >
                Review submissions →
              </button>
            </div>
            {pendingLoading && <p className="muted">Loading…</p>}
            {!pendingLoading && pending.length === 0 && <p className="muted">No pending submissions. 🎉</p>}
            {!pendingLoading && pending.length > 0 && pending.map(p => (
              <div key={p.id} style={{ display: 'flex', alignItems: 'center', gap: '.75rem', padding: '.6rem 0', borderBottom: '1px solid var(--border)' }}>
                <div style={{ flex: 1 }}>
                  <a href={p.repository} target="_blank" rel="noreferrer" style={{ fontWeight: 600 }}>{p.name}</a>
                  <span className="muted" style={{ fontSize: 'var(--fs-xs)', marginLeft: '.5rem' }}>by {p.author}</span>
                  <div className="muted" style={{ fontSize: 'var(--fs-xs)' }}>{p.description}</div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </>
  );
}
