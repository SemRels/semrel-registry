import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { hasToken } from '../lib/api';

type Plugin = {
  id: number;
  namespace?: string;
  name: string;
  description: string;
  author: string;
  category: string;
  repository: string;
  license: string;
  tags: string[];
};

type Pagination = { total: number; page: number; limit: number; pages: number };

const CAT_CLASS: Record<string, string> = {
  provider: 'badge--provider', analyzer: 'badge--analyzer',
  condition: 'badge--condition', hook: 'badge--hook', updater: 'badge--updater',
  generator: 'badge--generator',
};

const CATEGORIES = ['provider', 'analyzer', 'condition', 'hook', 'updater', 'generator'];
const SORTS = [
  { value: '', label: 'Default' },
  { value: 'name:asc', label: 'Name (A → Z)' },
  { value: 'name:desc', label: 'Name (Z → A)' },
  { value: 'updated_at:desc', label: 'Recently updated' },
  { value: 'created_at:desc', label: 'Newest' },
];

export default function RegistryPage() {
  const [plugins, setPlugins]         = useState<Plugin[]>([]);
  const [versions, setVersions]       = useState<Record<string, string>>({});
  const [pagination, setPagination]   = useState<Pagination | null>(null);
  const [search, setSearch]           = useState('');
  const [category, setCategory]       = useState('');
  const [sort, setSort]               = useState('');
  const [page, setPage]               = useState(1);
  const [loading, setLoading]         = useState(true);
  const [error, setError]             = useState('');

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setVersions({});

    const params = new URLSearchParams({ limit: '24', page: String(page) });
    if (search)   params.set('search', search);
    if (category) params.set('category', category);
    if (sort) {
      const [field, dir] = sort.split(':');
      params.set('sort', field);
      params.set('order', dir ?? 'asc');
    }

    async function load() {
      try {
        const d = await fetch(`/api/v1/plugins?${params}`).then(r => r.json());
        if (cancelled) return;
        setPlugins(d.data ?? []);
        setPagination(d.pagination ?? null);
        setError('');
        setLoading(false);

        // Progressively load latest version for each plugin
        for (const p of (d.data ?? []) as Plugin[]) {
          if (cancelled) break;
          const key = p.namespace ? `${p.namespace}/${p.name}` : p.name;
          fetch(`/api/v1/plugins/${encodeURIComponent(key)}/versions?limit=1`)
            .then(r => r.json())
            .then(vr => {
              if (cancelled) return;
              const latest = (vr.data ?? []).find((v: { prerelease: boolean }) => !v.prerelease)
                ?? vr.data?.[0];
              if (latest?.version) {
                setVersions(prev => ({ ...prev, [key]: latest.version }));
              }
            })
            .catch(() => { /* best-effort */ });
        }
      } catch {
        if (!cancelled) {
          setError('Failed to load plugins.');
          setLoading(false);
        }
      }
    }

    load();
    return () => { cancelled = true; };
  }, [page, search, category, sort]);

  // debounce search
  const [searchInput, setSearchInput] = useState('');
  useEffect(() => {
    const t = setTimeout(() => { setSearch(searchInput); setPage(1); }, 300);
    return () => clearTimeout(t);
  }, [searchInput]);

  const isLoggedIn = hasToken();

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg)', color: 'var(--fg)' }}>
      {/* Top bar */}
      <header style={{ borderBottom: '1px solid var(--border)', padding: '0 1.5rem', height: '3.25rem', display: 'flex', alignItems: 'center', justifyContent: 'space-between', position: 'sticky', top: 0, background: 'var(--bg)', zIndex: 10 }}>
        <a style={{ display: 'flex', alignItems: 'center', gap: '.5rem', textDecoration: 'none', color: 'var(--fg)', fontWeight: 700 }} href="/">
          <img src="/semrel.svg" alt="semrel" style={{ width: '1.4rem', height: '1.4rem' }} />
          semrel Registry
        </a>
        <div style={{ display: 'flex', gap: '.5rem', alignItems: 'center' }}>
          <a href="/api/v1/plugins" target="_blank" rel="noopener" className="btn btn--secondary" style={{ fontSize: 'var(--fs-sm)', padding: '4px 10px' }}>
            API ↗
          </a>
          {isLoggedIn
            ? <Link to="/admin" className="btn btn--primary" style={{ fontSize: 'var(--fs-sm)', padding: '4px 12px' }}>Admin Panel</Link>
            : <Link to="/login" className="btn btn--primary" style={{ fontSize: 'var(--fs-sm)', padding: '4px 12px' }}>Sign In</Link>
          }
        </div>
      </header>

      <div style={{ maxWidth: '1100px', margin: '0 auto', padding: '2rem 1.5rem' }}>
        {/* Hero */}
        <div style={{ textAlign: 'center', marginBottom: '2.5rem' }}>
          <h1 style={{ fontSize: 'clamp(1.5rem,4vw,2.25rem)', fontWeight: 800, marginBottom: '.5rem' }}>
            semrel Plugin Registry
          </h1>
          <p className="muted" style={{ fontSize: 'var(--fs-md)', marginBottom: '1.5rem' }}>
            Discover and install plugins for <a href="https://semrel.io" target="_blank" rel="noopener" style={{ color: 'var(--accent)' }}>semrel</a> — semantic versioning made simple.
          </p>
          {pagination && (
            <div style={{ display: 'flex', justifyContent: 'center', gap: '1.5rem' }}>
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: '1.75rem', fontWeight: 700, color: 'var(--accent)' }}>{pagination.total}</div>
                <div className="muted" style={{ fontSize: 'var(--fs-xs)' }}>Plugins</div>
              </div>
            </div>
          )}
        </div>

        {/* Search + filter + sort */}
        <div style={{ display: 'flex', gap: '.75rem', marginBottom: '1.5rem', flexWrap: 'wrap' }}>
          <input
            className="input"
            style={{ flex: 1, minWidth: '200px' }}
            placeholder="Search plugins…"
            value={searchInput}
            onChange={e => setSearchInput(e.target.value)}
          />
          <select
            className="input"
            style={{ width: 'auto' }}
            value={category}
            onChange={e => { setCategory(e.target.value); setPage(1); }}
          >
            <option value="">All categories</option>
            {CATEGORIES.map(c => <option key={c} value={c}>{c}</option>)}
          </select>
          <select
            className="input"
            style={{ width: 'auto' }}
            value={sort}
            onChange={e => { setSort(e.target.value); setPage(1); }}
          >
            {SORTS.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
          </select>
        </div>

        {/* Error */}
        {error && <div className="alert alert--error">{error}</div>}

        {/* Plugin grid */}
        {loading ? (
          <p className="muted" style={{ textAlign: 'center', padding: '3rem 0' }}>Loading…</p>
        ) : plugins.length === 0 ? (
          <p className="muted" style={{ textAlign: 'center', padding: '3rem 0' }}>No plugins found.</p>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))', gap: '1rem', marginBottom: '2rem' }}>
            {plugins.map(p => {
              const pluginKey = p.namespace ? `${p.namespace}/${p.name}` : p.name;
              return (
              <Link
                key={p.id}
                to={`/plugins/${encodeURIComponent(pluginKey)}`}
                style={{ textDecoration: 'none', color: 'inherit', display: 'flex' }}
              >
                <div className="card" style={{ padding: '1rem', display: 'flex', flexDirection: 'column', gap: '.4rem', width: '100%', cursor: 'pointer', transition: 'border-color .15s' }}
                  onMouseEnter={e => (e.currentTarget.style.borderColor = 'var(--accent)')}
                  onMouseLeave={e => (e.currentTarget.style.borderColor = '')}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '.5rem' }}>
                    <div style={{ overflow: 'hidden' }}>
                      {p.namespace && <span className="muted" style={{ fontSize: 'var(--fs-xs)', display: 'block' }}>{p.namespace}</span>}
                      <span style={{ fontWeight: 700, fontSize: 'var(--fs-md)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'block' }}>{p.name}</span>
                    </div>
                    <span className={`badge ${CAT_CLASS[p.category] ?? ''}`} style={{ flexShrink: 0 }}>{p.category}</span>
                  </div>
                  <p className="muted" style={{ fontSize: 'var(--fs-sm)', margin: 0, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
                    {p.description || 'No description.'}
                  </p>

                  {/* Install hint */}
                  <code style={{ fontSize: '11px', background: 'var(--surface2)', padding: '3px 7px', borderRadius: 5, color: 'var(--muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'block' }}>
                    semrel plugin install {pluginKey}
                  </code>

                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginTop: 'auto', paddingTop: '.25rem' }}>
                    <span className="muted" style={{ fontSize: 'var(--fs-xs)' }}>by {p.author}</span>
                    <div style={{ display: 'flex', gap: '.5rem', alignItems: 'center' }}>
                      {versions[pluginKey] && (() => {
                        const ver = versions[pluginKey];
                        const isDev = ver.startsWith('0.');
                        return (
                          <span style={{ fontSize: '11px', fontFamily: 'monospace', background: isDev ? 'rgba(210,153,34,.15)' : 'rgba(56,139,253,.12)', color: isDev ? '#d7a22a' : 'var(--accent)', borderRadius: 5, padding: '1px 7px', fontWeight: 600 }}>
                            v{ver}{isDev && <span style={{ marginLeft: '.2rem', fontSize: '9px', opacity: .75 }}>dev</span>}
                          </span>
                        );
                      })()}
                      {p.repository && (
                        <a
                          href={p.repository}
                          target="_blank"
                          rel="noopener"
                          style={{ fontSize: 'var(--fs-xs)', color: 'var(--muted)' }}
                          onClick={e => e.stopPropagation()}
                        >
                          GitHub ↗
                        </a>
                      )}
                    </div>
                  </div>
                </div>
              </Link>
              );
            })}
          </div>
        )}

        {/* Pagination */}
        {pagination && pagination.pages > 1 && (
          <div style={{ display: 'flex', justifyContent: 'center', gap: '.5rem' }}>
            <button className="btn btn--secondary" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>← Prev</button>
            <span className="muted" style={{ lineHeight: '2rem', fontSize: 'var(--fs-sm)' }}>Page {page} / {pagination.pages}</span>
            <button className="btn btn--secondary" disabled={page >= pagination.pages} onClick={() => setPage(p => p + 1)}>Next →</button>
          </div>
        )}

        {/* Footer */}
        <footer style={{ marginTop: '3rem', paddingTop: '1.5rem', borderTop: '1px solid var(--border)', display: 'flex', justifyContent: 'space-between', flexWrap: 'wrap', gap: '.5rem' }}>
          <span className="muted" style={{ fontSize: 'var(--fs-xs)' }}>© semrel · Plugin Registry</span>
          <div style={{ display: 'flex', gap: '1rem' }}>
            <a href="/api/v1/plugins" target="_blank" rel="noopener" className="muted" style={{ fontSize: 'var(--fs-xs)' }}>API</a>
            <a href="https://github.com/SemRels" target="_blank" rel="noopener" className="muted" style={{ fontSize: 'var(--fs-xs)' }}>GitHub</a>
            {!isLoggedIn && <Link to="/login" className="muted" style={{ fontSize: 'var(--fs-xs)' }}>Admin</Link>}
          </div>
        </footer>
      </div>
    </div>
  );
}
