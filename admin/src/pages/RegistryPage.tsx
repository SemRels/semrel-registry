import { useCallback, useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useCurrentUser } from '../hooks/useCurrentUser';
import ThemeToggle from '../components/ThemeToggle';
import CopyButton from '../components/CopyButton';

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
  latestVersion?: string;
  views?: number;
  downloads?: number;
};

type Pagination = { total: number; page: number; limit: number; pages: number };

const CAT_CLASS: Record<string, string> = {
  provider: 'badge--provider', analyzer: 'badge--analyzer',
  condition: 'badge--condition', hook: 'badge--hook', updater: 'badge--updater',
  generator: 'badge--generator', packager: 'badge--packager',
  publisher: 'badge--publisher',
};

const CATEGORIES = ['provider', 'analyzer', 'condition', 'hook', 'updater', 'generator', 'packager', 'publisher'];
const SORTS = [
  { value: '', label: 'Default' },
  { value: 'name:asc', label: 'Name (A → Z)' },
  { value: 'name:desc', label: 'Name (Z → A)' },
  { value: 'downloads:desc', label: '↓ Most downloaded' },
  { value: 'views:desc', label: '↓ Most viewed' },
  { value: 'updated_at:desc', label: 'Recently updated' },
  { value: 'created_at:desc', label: 'Newest' },
];

export default function RegistryPage() {
  // Filters live in the URL, not in component state. Previously a search, a
  // category and a page number existed only in memory: the result could not be
  // linked to or bookmarked, and the browser's back button left the list
  // instead of stepping back through it.
  const [searchParams, setSearchParams] = useSearchParams();
  const search   = searchParams.get('search')   ?? '';
  const category = searchParams.get('category') ?? '';
  const sort     = searchParams.get('sort')     ?? '';
  const page     = Math.max(1, Number.parseInt(searchParams.get('page') ?? '1', 10) || 1);

  const [plugins, setPlugins]       = useState<Plugin[]>([]);
  const [pagination, setPagination] = useState<Pagination | null>(null);
  const [loading, setLoading]       = useState(true);
  const [error, setError]           = useState('');
  const [reloadToken, setReloadToken] = useState(0);

  /** Writes a filter change to the URL, resetting to page 1 unless paging. */
  const updateParams = useCallback((changes: Record<string, string>, keepPage = false) => {
    setSearchParams(current => {
      const next = new URLSearchParams(current);
      for (const [key, value] of Object.entries(changes)) {
        if (value) next.set(key, value);
        else next.delete(key);
      }
      if (!keepPage) next.delete('page');
      return next;
    }, { replace: true });
  }, [setSearchParams]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

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
        const resp = await fetch(`/api/v1/plugins?${params}`);
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        const d = await resp.json();
        if (cancelled) return;
        setPlugins(d.data ?? []);
        setPagination(d.pagination ?? null);
        setError('');
        setLoading(false);
      } catch {
        if (!cancelled) {
          setError('Could not load plugins. The registry API may be unavailable.');
          setLoading(false);
        }
      }
    }

    void load();
    return () => { cancelled = true; };
  }, [page, search, category, sort, reloadToken]);

  // Debounced search input, seeded from the URL so a shared link shows its
  // own search term in the box.
  const [searchInput, setSearchInput] = useState(search);
  useEffect(() => {
    if (searchInput === search) return;
    const t = setTimeout(() => updateParams({ search: searchInput }), 300);
    return () => clearTimeout(t);
  }, [searchInput, search, updateParams]);

  const { user } = useCurrentUser();
  const isLoggedIn = user !== null;
  const hasFilters = Boolean(search || category);

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg)', color: 'var(--fg)' }}>
      <a className="skip-link" href="#main-content">Skip to main content</a>

      {/* Top bar */}
      <header style={{ borderBottom: '1px solid var(--border)', padding: '0 1.5rem', height: '3.25rem', display: 'flex', alignItems: 'center', justifyContent: 'space-between', position: 'sticky', top: 0, background: 'var(--bg)', zIndex: 10 }}>
        <a style={{ display: 'flex', alignItems: 'center', gap: '.5rem', textDecoration: 'none', color: 'var(--fg)', fontWeight: 700 }} href="/">
          <img src="/semrel.svg" alt="" aria-hidden="true" style={{ width: '1.4rem', height: '1.4rem' }} />
          semrel Registry
        </a>
        <div style={{ display: 'flex', gap: '.5rem', alignItems: 'center' }}>
          <ThemeToggle />
          <a href="/api/v1/plugins" target="_blank" rel="noopener noreferrer" className="btn btn--secondary" style={{ fontSize: 'var(--fs-sm)', padding: '4px 10px' }}>
            API <span aria-hidden="true">↗</span><span className="sr-only">(opens in a new tab)</span>
          </a>
          {isLoggedIn
            ? <Link to="/admin" className="btn btn--primary" style={{ fontSize: 'var(--fs-sm)', padding: '4px 12px' }}>Admin Panel</Link>
            : <Link to="/login" className="btn btn--primary" style={{ fontSize: 'var(--fs-sm)', padding: '4px 12px' }}>Sign In</Link>
          }
        </div>
      </header>

      <main id="main-content" tabIndex={-1} style={{ maxWidth: '1100px', margin: '0 auto', padding: '2rem 1.5rem' }}>
        {/* Hero */}
        <div style={{ textAlign: 'center', marginBottom: '2.5rem' }}>
          <h1 style={{ fontSize: 'clamp(1.5rem,4vw,2.25rem)', fontWeight: 800, marginBottom: '.5rem' }}>
            semrel Plugin Registry
          </h1>
          <p className="muted" style={{ fontSize: 'var(--fs-md)', marginBottom: '1.5rem' }}>
            Discover and install plugins for <a href="https://semrel.io" target="_blank" rel="noopener" style={{ color: 'var(--accent-text)' }}>semrel</a> — semantic versioning made simple.
          </p>
          {pagination && (
            <div style={{ display: 'flex', justifyContent: 'center', gap: '1.5rem' }}>
              <div style={{ textAlign: 'center' }}>
                <div style={{ fontSize: '1.75rem', fontWeight: 700, color: 'var(--accent-text)' }}>{pagination.total}</div>
                <div className="muted" style={{ fontSize: 'var(--fs-xs)' }}>Plugins</div>
              </div>
            </div>
          )}
        </div>

        {/* Search + filter + sort. Every control is labelled: a placeholder is
            not a label — it disappears on typing and several screen readers
            never announce it. */}
        <search>
          <div style={{ display: 'flex', gap: '.75rem', marginBottom: '1.5rem', flexWrap: 'wrap' }}>
            <div style={{ flex: 1, minWidth: '200px' }}>
              <label className="sr-only" htmlFor="registry-search">Search plugins</label>
              <input
                id="registry-search"
                type="search"
                className="input"
                style={{ width: '100%' }}
                placeholder="Search plugins…"
                value={searchInput}
                onChange={e => setSearchInput(e.target.value)}
              />
            </div>
            <div>
              <label className="sr-only" htmlFor="registry-category">Filter by category</label>
              <select
                id="registry-category"
                className="input"
                style={{ width: 'auto' }}
                value={category}
                onChange={e => updateParams({ category: e.target.value })}
              >
                <option value="">All categories</option>
                {CATEGORIES.map(c => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
            <div>
              <label className="sr-only" htmlFor="registry-sort">Sort by</label>
              <select
                id="registry-sort"
                className="input"
                style={{ width: 'auto' }}
                value={sort}
                onChange={e => updateParams({ sort: e.target.value })}
              >
                {SORTS.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
              </select>
            </div>
          </div>
        </search>

        {/* Error — with a way out, rather than a dead end. */}
        {error && (
          <div className="alert alert--error" role="alert">
            <span>{error}</span>
            <button
              type="button"
              className="btn btn--secondary btn--sm"
              style={{ marginLeft: '.75rem' }}
              onClick={() => setReloadToken(t => t + 1)}
            >
              Try again
            </button>
          </div>
        )}

        {/* Result count, announced when it changes. */}
        <div className="sr-only" role="status" aria-live="polite">
          {loading ? 'Loading plugins' : `${pagination?.total ?? plugins.length} plugins found`}
        </div>

        {/* Plugin grid */}
        {loading ? (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))', gap: '1rem', marginBottom: '2rem' }} aria-hidden="true">
            {Array.from({ length: 6 }, (_, i) => <div key={i} className="skeleton skeleton-card" />)}
          </div>
        ) : plugins.length === 0 ? (
          <div className="empty-state">
            <span className="empty-state__title">
              {hasFilters ? 'No plugins match these filters' : 'No plugins published yet'}
            </span>
            <p style={{ margin: 0, maxWidth: '32rem' }}>
              {hasFilters
                ? 'Try a broader search term or a different category.'
                : 'Once a plugin is published and approved it appears here.'}
            </p>
            {hasFilters && (
              <button type="button" className="btn btn--secondary" onClick={() => setSearchParams(new URLSearchParams(), { replace: true })}>
                Clear filters
              </button>
            )}
          </div>
        ) : (
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill,minmax(280px,1fr))', gap: '1rem', marginBottom: '2rem' }}>
            {plugins.map(p => {
              const pluginKey = p.namespace ? `${p.namespace}/${p.name}` : p.name;
              // The card is a container with a stretched link over the title
              // rather than a link wrapping everything: an interactive control
              // (the copy button) nested inside an anchor is invalid markup and
              // unreachable by keyboard in some browsers.
              return (
              <div
                key={p.id}
                className="card plugin-card"
                style={{ padding: '1rem', display: 'flex', flexDirection: 'column', gap: '.4rem', width: '100%' }}
              >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '.5rem' }}>
                    <div style={{ overflow: 'hidden' }}>
                      {p.namespace && <span className="muted" style={{ fontSize: 'var(--fs-xs)', display: 'block' }}>{p.namespace}</span>}
                      <Link
                        to={`/plugins/${encodeURIComponent(pluginKey)}`}
                        className="plugin-card__link"
                        style={{ fontWeight: 700, fontSize: 'var(--fs-md)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'block', color: 'inherit' }}
                      >
                        {p.name}
                      </Link>
                    </div>
                    <span className={`badge ${CAT_CLASS[p.category] ?? ''}`} style={{ flexShrink: 0 }}>{p.category}</span>
                  </div>
                  <p className="muted" style={{ fontSize: 'var(--fs-sm)', margin: 0, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>
                    {p.description || 'No description.'}
                  </p>

                  {/* Install command — the reason most visitors open this page,
                      so it is copyable without a detour through the detail view. */}
                  <div className="registry-cli-cmd">
                    <code>semrel plugin install {pluginKey}</code>
                    <CopyButton
                      text={`semrel plugin install ${pluginKey}`}
                      label="Copy install command"
                      className="copy-button copy-button--inline"
                    />
                  </div>

                  <div style={{ display: 'flex', alignItems: 'center', gap: '.35rem', flexWrap: 'wrap', marginTop: 'auto', paddingTop: '.25rem' }}>
                    <span className="muted" style={{ fontSize: 'var(--fs-xs)', flex: '1 1 auto', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>by {p.author}</span>
                    <div style={{ display: 'flex', gap: '.3rem', alignItems: 'center', flexShrink: 0 }}>
                      <span
                        style={{ fontSize: '11px', background: 'var(--success-soft)', color: 'var(--success)', borderRadius: 5, padding: '1px 6px', fontWeight: 600, whiteSpace: 'nowrap' }}
                        title="Total downloads"
                      >
                        ↓ {Number(p.downloads ?? 0).toLocaleString()}
                      </span>
                      <span
                        style={{ fontSize: '11px', background: 'var(--accent-soft)', color: 'var(--accent-text)', borderRadius: 5, padding: '1px 6px', fontWeight: 600, whiteSpace: 'nowrap' }}
                        title="Total views"
                      >
                        👁 {Number(p.views ?? 0).toLocaleString()}
                      </span>
                      {p.latestVersion && (() => {
                        const ver = p.latestVersion;
                        const isDev = ver.startsWith('0.');
                        return (
                          <span style={{ fontSize: '11px', fontFamily: 'monospace', background: isDev ? 'var(--warning-soft)' : 'var(--accent-soft)', color: isDev ? 'var(--warning)' : 'var(--accent)', borderRadius: 5, padding: '1px 6px', fontWeight: 600, whiteSpace: 'nowrap' }}>
                            <span className="sr-only">{isDev ? 'Development version ' : 'Latest version '}</span>v{ver}
                          </span>
                        );
                      })()}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Pagination */}
        {pagination && pagination.pages > 1 && (
          <nav aria-label="Pagination" style={{ display: 'flex', justifyContent: 'center', gap: '.5rem' }}>
            <button
              type="button"
              className="btn btn--secondary"
              disabled={page <= 1}
              onClick={() => updateParams({ page: String(page - 1) }, true)}
            >
              <span aria-hidden="true">←</span> Previous
            </button>
            <span className="muted" style={{ lineHeight: '2rem', fontSize: 'var(--fs-sm)' }}>
              Page {page} of {pagination.pages}
            </span>
            <button
              type="button"
              className="btn btn--secondary"
              disabled={page >= pagination.pages}
              onClick={() => updateParams({ page: String(page + 1) }, true)}
            >
              Next <span aria-hidden="true">→</span>
            </button>
          </nav>
        )}

        {/* Footer */}
        <footer style={{ marginTop: '3rem', paddingTop: '1.5rem', borderTop: '1px solid var(--border)', display: 'flex', justifyContent: 'space-between', flexWrap: 'wrap', gap: '.5rem' }}>
          <span className="muted" style={{ fontSize: 'var(--fs-xs)' }}>© semrel · Plugin Registry</span>
          <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap' }}>
            <a href="/api/v1/plugins" target="_blank" rel="noopener" className="muted" style={{ fontSize: 'var(--fs-xs)' }}>API</a>
            <a href="https://semrel.io" target="_blank" rel="noopener" className="muted" style={{ fontSize: 'var(--fs-xs)' }}>Docs</a>
            <a href="https://github.com/SemRels" target="_blank" rel="noopener" className="muted" style={{ fontSize: 'var(--fs-xs)' }}>GitHub</a>
            <a href="https://semrel.io/legal/imprint/" target="_blank" rel="noopener" className="muted" style={{ fontSize: 'var(--fs-xs)' }}>Imprint</a>
            <a href="https://semrel.io/legal/privacy/" target="_blank" rel="noopener" className="muted" style={{ fontSize: 'var(--fs-xs)' }}>Privacy</a>
            {!isLoggedIn && <Link to="/login" className="muted" style={{ fontSize: 'var(--fs-xs)' }}>Admin</Link>}
          </div>
        </footer>
      </main>
    </div>
  );
}
