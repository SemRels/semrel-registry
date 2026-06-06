import { useEffect, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
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
  const [searchParams, setSearchParams] = useSearchParams();
  const initialPage = Number.parseInt(searchParams.get('page') ?? '1', 10);
  const initialSort = (searchParams.get('sort') as 'name' | 'downloads' | 'views' | null) ?? 'name';
  const initialOrder = (searchParams.get('order') as 'asc' | 'desc' | null) ?? 'asc';
  const [plugins, setPlugins]         = useState<Plugin[]>([]);
  const [pagination, setPagination]   = useState<Pagination | null>(null);
  const [search, setSearch]           = useState(searchParams.get('search') ?? '');
  const [category, setCategory]       = useState(searchParams.get('category') ?? '');
  const [sort, setSort]               = useState<'name' | 'downloads' | 'views'>(initialSort);
  const [order, setOrder]             = useState<'asc' | 'desc'>(initialOrder);
  const [page, setPage]               = useState(Number.isFinite(initialPage) && initialPage > 0 ? initialPage : 1);
  const [loading, setLoading]         = useState(false);
  const [error, setError]             = useState('');
  const navigate                      = useNavigate();

  useEffect(() => {
    const params = new URLSearchParams();
    if (search) params.set('search', search);
    if (category) params.set('category', category);
    if (sort !== 'name') params.set('sort', sort);
    if (order !== 'asc') params.set('order', order);
    if (page > 1) params.set('page', String(page));
    setSearchParams(params, { replace: true });
  }, [search, category, sort, order, page, setSearchParams]);

  useEffect(() => {
    if (!user) return;
    let cancelled = false;
    setLoading(true);
    const author = isAdmin ? undefined : (user.login || undefined);
    listPlugins({
      page,
      limit: 25,
      search: search || undefined,
      category: category || undefined,
      author,
      sort,
      order,
    })
      .then((res) => {
        if (cancelled) return;
        setPlugins(res.data ?? []);
        setPagination(res.pagination ?? null);
        setError('');
        setLoading(false);
      })
      .catch((e: unknown) => { if (!cancelled) { setError(e instanceof Error ? e.message : 'Failed'); setLoading(false); } });
    return () => { cancelled = true; };
  }, [page, search, category, sort, order, user, isAdmin]);

  async function handleDelete(p: Plugin) {
    if (!globalThis.confirm(`Delete "${p.name}"?`)) return;
    try { await deletePlugin(p.id); setPlugins((prev) => prev.filter((x) => x.id !== p.id)); }
    catch (e: unknown) { alert(e instanceof Error ? e.message : 'Delete failed'); }
  }

  // Non-admins can only edit/delete their own plugins.
  function canEdit(p: Plugin) {
    return isAdmin || (p.author?.toLowerCase() === user?.login?.toLowerCase());
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
          <select className="select" style={{ width: 180 }} value={`${sort}:${order}`}
            onChange={(e) => {
              const [nextSort, nextOrder] = e.target.value.split(':') as ['name' | 'downloads' | 'views', 'asc' | 'desc'];
              setSort(nextSort);
              setOrder(nextOrder);
              setPage(1);
            }}>
            <option value="name:asc">Name A-Z</option>
            <option value="name:desc">Name Z-A</option>
            <option value="downloads:desc">Downloads high-low</option>
            <option value="downloads:asc">Downloads low-high</option>
            <option value="views:desc">Views high-low</option>
            <option value="views:asc">Views low-high</option>
          </select>
        </div>

        {loading ? <p className="muted">Loading…</p> : (
          <div className="table-wrap">
            <table className="table--stack">
              <thead><tr>
                <th>Name</th><th>Category</th><th>Author</th><th>License</th><th>Latest</th><th>Views</th><th>Downloads</th><th>Status</th><th></th>
              </tr></thead>
              <tbody>
                {plugins.length === 0 && (
                  <tr><td colSpan={9} style={{ textAlign:'center', padding:'2rem' }} className="muted">
                    {isAdmin ? <Link to="/admin/plugins/new">Add the first plugin</Link> : 'No plugins attributed to your account yet.'}
                  </td></tr>
                )}
                {plugins.map((p) => (
                  <tr key={p.id}>
                    <td data-label="Name"><strong style={{ fontSize:'var(--fs-sm)' }}>{p.namespace ? <span className="muted" style={{ fontWeight:400 }}>{p.namespace}/</span> : null}{p.name}</strong>
                      {p.description && <div className="muted truncate" style={{ fontSize:'var(--fs-xs)', maxWidth:240 }}>{p.description}</div>}
                    </td>
                    <td data-label="Category"><span className={`badge ${CAT_CLASS[p.category] ?? ''}`}>{p.category}</span></td>
                    <td data-label="Author" className="muted" style={{ fontSize:'var(--fs-sm)' }}>{p.author}</td>
                    <td data-label="License" className="muted" style={{ fontSize:'var(--fs-sm)' }}>{p.license}</td>
                    <td data-label="Latest" style={{ fontSize:'var(--fs-sm)' }}>{p.latestVersion ? <code>v{p.latestVersion}</code> : <span className="muted">—</span>}</td>
                    <td data-label="Views" style={{ fontSize:'var(--fs-sm)' }}>{Number(p.views ?? 0).toLocaleString()}</td>
                    <td data-label="Downloads" style={{ fontSize:'var(--fs-sm)' }}>{Number(p.downloads ?? 0).toLocaleString()}</td>
                    <td data-label="Status">
                      {p.status !== 'active' && (
                        <span style={{
                          display:'inline-block', padding:'1px 7px', borderRadius:4,
                          fontSize:'var(--fs-xs)', fontWeight:600, marginRight:'.4rem',
                          background: p.status === 'pending' ? 'rgba(210,153,34,.2)' : 'rgba(248,81,73,.15)',
                          color: p.status === 'pending' ? '#d29922' : '#f85149',
                        }}>{p.status}</span>
                      )}
                    </td>
                    <td data-label="Actions"><div className="flex gap-sm">
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
