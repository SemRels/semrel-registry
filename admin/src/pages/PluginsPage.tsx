import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { listPlugins, deletePlugin } from '../lib/api';
import type { Plugin, Pagination } from '../lib/api';

const CATEGORY_COLORS: Record<string, string> = {
  provider: 'badge-provider',
  analyzer: 'badge-analyzer',
  condition: 'badge-condition',
  hook: 'badge-hook',
  updater: 'badge-updater',
};

export default function PluginsPage() {
  const [plugins, setPlugins] = useState<Plugin[]>([]);
  const [pagination, setPagination] = useState<Pagination | null>(null);
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState('');
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const navigate = useNavigate();

  useEffect(() => {
    setLoading(true);
    listPlugins({ page, limit: 20, search: search || undefined, category: category || undefined })
      .then((res) => {
        setPlugins(res.data ?? []);
        setPagination(res.pagination ?? null);
        setError('');
      })
      .catch((e: unknown) => setError(e instanceof Error ? e.message : 'Failed to load plugins'))
      .finally(() => setLoading(false));
  }, [page, search, category]);

  async function handleDelete(plugin: Plugin) {
    if (!window.confirm(`Delete plugin "${plugin.name}"? This cannot be undone.`)) return;
    try {
      await deletePlugin(plugin.id);
      setPlugins((prev) => prev.filter((p) => p.id !== plugin.id));
    } catch (e: unknown) {
      alert(e instanceof Error ? e.message : 'Delete failed');
    }
  }

  return (
    <>
      <div className="flex-between" style={{ marginBottom: '0.25rem' }}>
        <h1 className="page-title">Plugins</h1>
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => navigate('/plugins/new')}
        >
          + Add Plugin
        </button>
      </div>
      <p className="page-subtitle">Manage registered plugins in the registry.</p>

      <div className="flex-row" style={{ marginBottom: '1.25rem' }}>
        <input
          type="search"
          className="form-input"
          style={{ maxWidth: 320 }}
          placeholder="Search by name, author…"
          value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(1); }}
        />
        <select
          className="form-select"
          style={{ maxWidth: 180 }}
          value={category}
          onChange={(e) => { setCategory(e.target.value); setPage(1); }}
        >
          <option value="">All categories</option>
          <option value="provider">Provider</option>
          <option value="analyzer">Analyzer</option>
          <option value="condition">Condition</option>
          <option value="hook">Hook</option>
          <option value="updater">Updater</option>
        </select>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      {loading ? (
        <div className="loading-block"><span className="spinner" /> Loading…</div>
      ) : (
        <div className="card" style={{ padding: 0 }}>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Category</th>
                  <th>Author</th>
                  <th>License</th>
                  <th>Latest</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {plugins.length === 0 && (
                  <tr>
                    <td colSpan={6} style={{ textAlign: 'center', padding: '2rem', color: 'var(--text-muted)' }}>
                      No plugins found.{' '}
                      <Link to="/plugins/new">Add one?</Link>
                    </td>
                  </tr>
                )}
                {plugins.map((plugin) => (
                  <tr key={plugin.id}>
                    <td>
                      <strong>{plugin.name}</strong>
                      {plugin.description && (
                        <div className="text-sm text-muted" style={{ marginTop: 2 }}>
                          {plugin.description.slice(0, 60)}{plugin.description.length > 60 ? '…' : ''}
                        </div>
                      )}
                    </td>
                    <td>
                      <span className={`badge ${CATEGORY_COLORS[plugin.category] ?? ''}`}>
                        {plugin.category}
                      </span>
                    </td>
                    <td className="text-muted text-sm">{plugin.author}</td>
                    <td className="text-muted text-sm">{plugin.license}</td>
                    <td className="text-sm">
                      {plugin.latestVersion ? (
                        <code>v{plugin.latestVersion}</code>
                      ) : (
                        <span className="text-muted">—</span>
                      )}
                    </td>
                    <td>
                      <div className="flex-row">
                        <Link to={`/plugins/${plugin.id}`} className="btn btn-ghost btn-sm">
                          Edit
                        </Link>
                        <Link to={`/plugins/${plugin.id}/versions`} className="btn btn-ghost btn-sm">
                          Versions
                        </Link>
                        <button
                          type="button"
                          className="btn btn-danger btn-sm"
                          onClick={() => { void handleDelete(plugin); }}
                        >
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {pagination && pagination.pages > 1 && (
        <div className="flex-row" style={{ marginTop: '1rem', justifyContent: 'center' }}>
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            ← Previous
          </button>
          <span className="text-sm text-muted">
            Page {pagination.page} of {pagination.pages} ({pagination.total} total)
          </span>
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            disabled={page >= pagination.pages}
            onClick={() => setPage((p) => p + 1)}
          >
            Next →
          </button>
        </div>
      )}
    </>
  );
}
