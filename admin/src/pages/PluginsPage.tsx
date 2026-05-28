import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { listPlugins, deletePlugin } from '../lib/api';
import type { Plugin, Pagination } from '../lib/api';
import { useCurrentUser } from '../hooks/useCurrentUser';

const CAT_CLASS: Record<string, string> = {
  provider: 'badge--provider', analyzer: 'badge--analyzer',
  condition: 'badge--condition', hook: 'badge--hook', updater: 'badge--updater',
};

export default function PluginsPage() {
  const user                          = useCurrentUser();
  const isAdmin                       = user?.isAdmin ?? false;
  const [plugins, setPlugins]         = useState<Plugin[]>([]);
  const [versions, setVersions]       = useState<Record<string, string>>({});
  const [pagination, setPagination]   = useState<Pagination | null>(null);
  const [search, setSearch]           = useState('');
  const [category, setCategory]       = useState('');
  const [page, setPage]               = useState(1);
  const [loading, setLoading]         = useState(false);
  const [error, setError]             = useState('');
  const navigate                      = useNavigate();

  useEffect(() => {
    if (!user) return;
    let cancelled = false;
    setLoading(true);
    setVersions({});
    const author = isAdmin ? undefined : (user.login || undefined);
    listPlugins({ page, limit: 25, search: search || undefined, category: category || undefined, author })
      .then((res) => {
        if (cancelled) return;
        const list = res.data ?? [];
        setPlugins(list);
        setPagination(res.pagination ?? null);
        setError('');
        setLoading(false);
        // Progressively load latest version for each plugin
        for (const p of list) {
          if (cancelled) break;
          fetch(`/api/v1/plugins/${p.name}/versions?limit=1`)
            .then(r => r.json())
            .then(vr => {
              if (cancelled) return;
              const latest = (vr.data ?? []).find((v: { prerelease: boolean }) => !v.prerelease) ?? vr.data?.[0];
              if (latest?.version) setVersions(prev => ({ ...prev, [p.name]: latest.version }));
            })
            .catch(() => {/* best-effort */});
        }
      })
      .catch((e: unknown) => { if (!cancelled) { setError(e instanceof Error ? e.message : 'Failed'); setLoading(false); } });
    return () => { cancelled = true; };
  }, [page, search, category, user, isAdmin]);

  async function handleDelete(p: Plugin) {
    if (!window.confirm(`Delete "${p.name}"?`)) return;
    try { await deletePlugin(p.id); setPlugins((prev) => prev.filter((x) => x.id !== p.id)); }
    catch (e: unknown) { alert(e instanceof Error ? e.message : 'Delete failed'); }
  }

  // Non-admins can only edit/delete their own plugins.
  function canEdit(p: Plugin) {
    return isAdmin || (user?.login && p.author?.toLowerCase() === user.login.toLowerCase());
  }

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">{isAdmin ? 'Plugins' : 'My Plugins'}</h1>
        <button type="button" className="btn btn--primary" onClick={() => navigate('/admin/plugins/new')}>+ Add</button>
      </div>
      <div className="page__body">
        {!isAdmin && (
          <div className="alert" style={{ background:'rgba(56,139,253,.1)', border:'1px solid rgba(56,139,253,.3)', color:'#79c0ff', padding:'.5rem .75rem', borderRadius:'6px', fontSize:'var(--fs-sm)', marginBottom:'.75rem' }}>
            Community view — you can only manage plugins attributed to <strong>{user?.login}</strong>.{' '}
            <a href="/submit" target="_blank" rel="noopener" style={{ color:'var(--accent)' }}>
              Submit a new plugin →
            </a>
          </div>
        )}
        {error && <div className="alert alert--error">{error}</div>}
        <div className="search-bar" style={{ marginBottom: '.75rem' }}>
          <input type="search" className="search-input" placeholder="Search…" value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }} />
          <select className="select" style={{ width: 160 }} value={category}
            onChange={(e) => { setCategory(e.target.value); setPage(1); }}>
            <option value="">All categories</option>
            {['provider','analyzer','condition','hook','updater'].map(c => <option key={c} value={c}>{c}</option>)}
          </select>
        </div>

        {loading ? <p className="muted">Loading…</p> : (
          <div className="table-wrap">
            <table>
              <thead><tr>
                <th>Name</th><th>Category</th><th>Author</th><th>License</th><th>Latest</th><th>Status</th><th></th>
              </tr></thead>
              <tbody>
                {plugins.length === 0 && (
                  <tr><td colSpan={7} style={{ textAlign:'center', padding:'2rem' }} className="muted">
                    {isAdmin ? <><Link to="/admin/plugins/new">Add the first plugin</Link></> : 'No plugins attributed to your account yet.'}
                  </td></tr>
                )}
                {plugins.map((p) => (
                  <tr key={p.id}>
                    <td><strong style={{ fontSize:'var(--fs-sm)' }}>{p.name}</strong>
                      {p.description && <div className="muted truncate" style={{ fontSize:'var(--fs-xs)', maxWidth:240 }}>{p.description}</div>}
                    </td>
                    <td><span className={`badge ${CAT_CLASS[p.category] ?? ''}`}>{p.category}</span></td>
                    <td className="muted" style={{ fontSize:'var(--fs-sm)' }}>{p.author}</td>
                    <td className="muted" style={{ fontSize:'var(--fs-sm)' }}>{p.license}</td>
                    <td style={{ fontSize:'var(--fs-sm)' }}>{versions[p.name] ? <code>v{versions[p.name]}</code> : <span className="muted">—</span>}</td>
                    <td>
                      {p.status !== 'active' && (
                        <span style={{
                          display:'inline-block', padding:'1px 7px', borderRadius:4,
                          fontSize:'var(--fs-xs)', fontWeight:600, marginRight:'.4rem',
                          background: p.status === 'pending' ? 'rgba(210,153,34,.2)' : 'rgba(248,81,73,.15)',
                          color: p.status === 'pending' ? '#d29922' : '#f85149',
                        }}>{p.status}</span>
                      )}
                    </td>
                    <td><div className="flex gap-sm">
                      {canEdit(p) ? (
                        <>
                          <Link to={`/admin/plugins/${p.id}`} className="btn btn--sm">Edit</Link>
                          <Link to={`/admin/plugins/${p.id}/versions`} className="btn btn--sm">Versions</Link>
                          <button type="button" className="btn btn--sm btn--danger" onClick={() => { void handleDelete(p); }}>Del</button>
                        </>
                      ) : (
                        <span className="muted" style={{ fontSize:'var(--fs-xs)' }}>read-only</span>
                      )}
                    </div></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {pagination && pagination.pages > 1 && (
          <div className="flex gap-sm mt-2" style={{ justifyContent:'center', alignItems:'center' }}>
            <button type="button" className="btn btn--sm" disabled={page <= 1} onClick={() => setPage(p => p-1)}>← Prev</button>
            <span className="muted" style={{ fontSize:'var(--fs-xs)' }}>Page {pagination.page}/{pagination.pages} ({pagination.total})</span>
            <button type="button" className="btn btn--sm" disabled={page >= pagination.pages} onClick={() => setPage(p => p+1)}>Next →</button>
          </div>
        )}
      </div>
    </>
  );
}
