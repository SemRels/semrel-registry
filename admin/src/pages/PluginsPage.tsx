import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { listPlugins, deletePlugin } from '../lib/api';
import type { Plugin, Pagination } from '../lib/api';

const CAT_CLASS: Record<string, string> = {
  provider: 'badge--provider', analyzer: 'badge--analyzer',
  condition: 'badge--condition', hook: 'badge--hook', updater: 'badge--updater',
};

export default function PluginsPage() {
  const [plugins, setPlugins]     = useState<Plugin[]>([]);
  const [pagination, setPagination] = useState<Pagination | null>(null);
  const [search, setSearch]       = useState('');
  const [category, setCategory]   = useState('');
  const [page, setPage]           = useState(1);
  const [loading, setLoading]     = useState(false);
  const [error, setError]         = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    setLoading(true);
    listPlugins({ page, limit: 25, search: search || undefined, category: category || undefined })
      .then((res) => { setPlugins(res.data ?? []); setPagination(res.pagination ?? null); setError(''); })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed'))
      .finally(() => setLoading(false));
  }, [page, search, category]);

  async function handleDelete(p: Plugin) {
    if (!window.confirm(`Delete "${p.name}"?`)) return;
    try { await deletePlugin(p.id); setPlugins((prev) => prev.filter((x) => x.id !== p.id)); }
    catch (e: unknown) { alert(e instanceof Error ? e.message : 'Delete failed'); }
  }

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">Plugins</h1>
        <button type="button" className="btn btn--primary" onClick={() => navigate('/plugins/new')}>+ Add</button>
      </div>
      <div className="page__body">
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
                <th>Name</th><th>Category</th><th>Author</th><th>License</th><th>Latest</th><th></th>
              </tr></thead>
              <tbody>
                {plugins.length === 0 && (
                  <tr><td colSpan={6} style={{ textAlign:'center', padding:'2rem' }} className="muted">
                    No plugins. <Link to="/plugins/new">Add one?</Link>
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
                    <td style={{ fontSize:'var(--fs-sm)' }}>{p.latestVersion ? <code>v{p.latestVersion}</code> : <span className="muted">—</span>}</td>
                    <td><div className="flex gap-sm">
                      <Link to={`/plugins/${p.id}`} className="btn btn--sm">Edit</Link>
                      <Link to={`/plugins/${p.id}/versions`} className="btn btn--sm">Versions</Link>
                      <button type="button" className="btn btn--sm btn--danger" onClick={() => { void handleDelete(p); }}>Del</button>
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
