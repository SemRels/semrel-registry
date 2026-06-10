import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getStats, syncFromFile, syncVersions, syncGitHubOrg, listPlugins } from '../lib/api';
import type { Stats, SyncResult, SyncVersionsResult, OrgSyncResult, Plugin } from '../lib/api';
import { useCurrentUser } from '../hooks/useCurrentUser';

type TrendPoint = { period: string; views: number; downloads: number };

function buildZeroSeries(range: 'day' | 'week' | 'month'): TrendPoint[] {
  const now = new Date();
  const out: TrendPoint[] = [];
  const count = range === 'day' ? 14 : 12;

  for (let i = count - 1; i >= 0; i--) {
    const d = new Date(now);
    if (range === 'day') {
      d.setDate(d.getDate() - i);
      out.push({ period: d.toISOString().slice(0, 10), views: 0, downloads: 0 });
      continue;
    }
    if (range === 'week') {
      d.setDate(d.getDate() - i * 7);
      const weekStart = new Date(d);
      const day = (weekStart.getDay() + 6) % 7;
      weekStart.setDate(weekStart.getDate() - day);
      const year = weekStart.getUTCFullYear();
      const firstThursday = new Date(Date.UTC(year, 0, 4));
      const firstWeekStart = new Date(firstThursday);
      firstWeekStart.setUTCDate(firstThursday.getUTCDate() - ((firstThursday.getUTCDay() + 6) % 7));
      const diff = Math.round((weekStart.getTime() - firstWeekStart.getTime()) / (7 * 24 * 3600 * 1000));
      const week = Math.max(1, diff + 1);
      out.push({ period: `${year}-W${String(week).padStart(2, '0')}`, views: 0, downloads: 0 });
      continue;
    }
    d.setMonth(d.getMonth() - i);
    out.push({ period: `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`, views: 0, downloads: 0 });
  }

  return out;
}

function linePath(values: number[], width: number, height: number, pad: number): string {
  if (values.length === 0) return '';
  const max = Math.max(1, ...values);
  const innerW = width - pad * 2;
  const innerH = height - pad * 2;
  return values.map((v, i) => {
    const x = pad + (values.length === 1 ? innerW / 2 : (i / (values.length - 1)) * innerW);
    const y = pad + innerH - (v / max) * innerH;
    return `${i === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`;
  }).join(' ');
}

function SeriesLineChart({
  data,
  hoveredIndex,
  onHoverIndexChange,
}: Readonly<{
  data: TrendPoint[];
  hoveredIndex: number | null;
  onHoverIndexChange: (index: number | null) => void;
}>) {
  const width = 760;
  const height = 220;
  const pad = 18;
  const ordered = [...data].reverse();
  const views = ordered.map((p) => p.views ?? 0);
  const downloads = ordered.map((p) => p.downloads ?? 0);
  const max = Math.max(1, ...views, ...downloads);
  const innerW = width - pad * 2;
  const innerH = height - pad * 2;
  const step = ordered.length <= 1 ? 0 : innerW / (ordered.length - 1);
  const pointX = (idx: number) => pad + (ordered.length === 1 ? innerW / 2 : idx * step);
  const pointY = (value: number) => pad + innerH - (value / max) * innerH;
  const viewPath = linePath(views, width, height, pad);
  const downloadPath = linePath(downloads, width, height, pad);
  const hoverX = hoveredIndex === null ? null : pointX(hoveredIndex);
  const hoverViewsY = hoveredIndex === null ? null : pointY(views[hoveredIndex] ?? 0);
  const hoverDownloadsY = hoveredIndex === null ? null : pointY(downloads[hoveredIndex] ?? 0);

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      width="100%"
      height="220"
      role="img"
      aria-label="Trend chart for views and downloads"
      onMouseLeave={() => onHoverIndexChange(null)}
    >
      <rect x="0" y="0" width={width} height={height} rx="10" fill="rgba(13,17,23,.25)" />
      <path d={viewPath} fill="none" stroke="rgba(56,139,253,.95)" strokeWidth="2.5" />
      <path d={downloadPath} fill="none" stroke="rgba(63,185,80,.95)" strokeWidth="2.5" />
      {hoverX !== null && (
        <>
          <line x1={hoverX} x2={hoverX} y1={pad} y2={height - pad} stroke="rgba(201,209,217,.4)" strokeDasharray="3 3" />
          <circle cx={hoverX} cy={hoverViewsY ?? 0} r="4" fill="rgba(56,139,253,.95)" />
          <circle cx={hoverX} cy={hoverDownloadsY ?? 0} r="4" fill="rgba(63,185,80,.95)" />
        </>
      )}
      {ordered.map((point, idx) => {
        const x = pointX(idx);
        const hit = Math.max(12, step || 24);
        return (
          <rect
            key={point.period}
            x={x - hit / 2}
            y={pad}
            width={hit}
            height={innerH}
            fill="transparent"
            onMouseEnter={() => onHoverIndexChange(idx)}
          />
        );
      })}
    </svg>
  );
}

function TopPluginsBarChart({ data }: Readonly<{ data: NonNullable<Stats['topPlugins']> }>) {
  if (!data || data.length === 0) return null;
  const max = Math.max(1, ...data.map((d) => Math.max(d.views, d.downloads)));
  return (
    <div style={{ display: 'grid', gap: '.6rem' }}>
      {data.slice(0, 6).map((item) => {
        const views = Number(item.views ?? 0);
        const downloads = Number(item.downloads ?? 0);
        const v = Math.max(2, Math.round((views / max) * 100));
        const d = Math.max(2, Math.round((downloads / max) * 100));
        return (
          <div key={item.pluginId} style={{ display: 'grid', gap: '.2rem' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: 'var(--fs-xs)' }}>
              <span>{item.namespace ? `${item.namespace}/` : ''}{item.name}</span>
              <span className="muted">V {views.toLocaleString()} · D {downloads.toLocaleString()}</span>
            </div>
            <div style={{ display: 'grid', gap: '.2rem' }}>
              <div style={{ width: `${v}%`, height: 6, borderRadius: 999, background: 'rgba(56,139,253,.7)' }} />
              <div style={{ width: `${d}%`, height: 6, borderRadius: 999, background: 'rgba(63,185,80,.75)' }} />
            </div>
          </div>
        );
      })}
    </div>
  );
}

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
  const [hoveredSeriesIndex, setHoveredSeriesIndex] = useState<number | null>(null);

  useEffect(() => {
    setHoveredSeriesIndex(null);
  }, [seriesRange, stats?.timestamp]);

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

  const rawSeries = stats?.series?.[seriesRange] ?? [];
  const zeroSeries = buildZeroSeries(seriesRange);
  // Merge sparse API data into zero-filled baseline so the chart always has N points.
  const activeSeries = (() => {
    if (rawSeries.length === 0) return zeroSeries;
    const map = new Map(rawSeries.map((p) => [p.period, p]));
    return zeroSeries.map((z) => map.get(z.period) ?? z);
  })();
  const categories = stats?.categories ?? {};
  const totalPlugins = Number(stats?.totalPlugins ?? 0);
  const totalViews = Number(stats?.totalViews ?? 0);
  const totalDownloads = Number(stats?.totalDownloads ?? 0);
  const topPlugins = stats?.topPlugins ?? [];
  const topVersions = stats?.topVersions ?? [];
  const orderedSeries = [...activeSeries].reverse();
  const activePoint = (() => {
    if (orderedSeries.length === 0) return null;
    if (hoveredSeriesIndex !== null && orderedSeries[hoveredSeriesIndex]) return orderedSeries[hoveredSeriesIndex];
    return orderedSeries[orderedSeries.length - 1] ?? null;
  })();

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
              <div className="stat-card__value">{totalPlugins}</div>
            </div>
            <div className="stat-card">
              <div className="stat-card__label">Views</div>
              <div className="stat-card__value">{totalViews.toLocaleString()}</div>
            </div>
            <div className="stat-card">
              <div className="stat-card__label">Downloads</div>
              <div className="stat-card__value">{totalDownloads.toLocaleString()}</div>
            </div>
            {Object.entries(categories).map(([cat, count]) => (
              <div key={cat} className="stat-card">
                <div className="stat-card__label">{cat}</div>
                <div className="stat-card__value">{Number(count ?? 0).toLocaleString()}</div>
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
            <div style={{ marginTop: '.75rem' }}>
              <SeriesLineChart
                data={activeSeries}
                hoveredIndex={hoveredSeriesIndex}
                onHoverIndexChange={setHoveredSeriesIndex}
              />
              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '1rem', marginTop: '.4rem', fontSize: 'var(--fs-xs)' }}>
                <span style={{ color: 'rgba(56,139,253,.95)' }}>Views</span>
                <span style={{ color: 'rgba(63,185,80,.95)' }}>Downloads</span>
              </div>
              {rawSeries.length === 0 && (
                <div style={{ marginTop: '.4rem', fontSize: 'var(--fs-xs)', color: 'var(--muted)' }}>
                  Noch keine Events vorhanden. Die Grafik zeigt aktuell eine 0-Basislinie.
                </div>
              )}
              {activePoint && (
                <div style={{ marginTop: '.5rem', fontSize: 'var(--fs-xs)', color: 'var(--muted)' }}>
                  <strong>{activePoint.period}</strong> · Views {Number(activePoint.views ?? 0).toLocaleString()} · Downloads {Number(activePoint.downloads ?? 0).toLocaleString()}
                </div>
              )}
            </div>
          </div>

          <div style={{ marginTop: '1rem', display: 'grid', gap: '1rem' }}>
            <div className="card">
              <h2 style={{ margin: 0, fontSize: 'var(--fs-md)', marginBottom: '.75rem' }}>Top plugins</h2>
              {topPlugins.length === 0 ? (
                <p className="muted">No plugin metrics yet.</p>
              ) : (
                <>
                <TopPluginsBarChart data={topPlugins} />
                <div className="table-wrap" style={{ marginTop: '.8rem' }}>
                  <table className="table--stack">
                    <thead><tr><th>Plugin</th><th>Category</th><th>Views</th><th>Downloads</th></tr></thead>
                    <tbody>
                      {topPlugins.map((item) => (
                        <tr key={item.pluginId}>
                          <td data-label="Plugin">{item.namespace ? `${item.namespace}/` : ''}{item.name}</td>
                          <td data-label="Category" className="muted">{item.category}</td>
                          <td data-label="Views">{Number(item.views ?? 0).toLocaleString()}</td>
                          <td data-label="Downloads">{Number(item.downloads ?? 0).toLocaleString()}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                </>
              )}
            </div>

            <div className="card">
              <h2 style={{ margin: 0, fontSize: 'var(--fs-md)', marginBottom: '.75rem' }}>Top versions</h2>
              {topVersions.length === 0 ? (
                <p className="muted">No version metrics yet.</p>
              ) : (
                <div className="table-wrap">
                  <table className="table--stack">
                    <thead><tr><th>Version</th><th>Plugin</th><th>Views</th><th>Downloads</th></tr></thead>
                    <tbody>
                      {topVersions.map((item) => (
                        <tr key={item.versionId}>
                          <td data-label="Version"><code>v{item.version}</code></td>
                          <td data-label="Plugin">{item.namespace ? `${item.namespace}/` : ''}{item.pluginName}</td>
                          <td data-label="Views">{Number(item.views ?? 0).toLocaleString()}</td>
                          <td data-label="Downloads">{Number(item.downloads ?? 0).toLocaleString()}</td>
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
