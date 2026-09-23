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
  /** Core range declared by the latest installable release. */
  latestSemrelCore?: string;
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
  const compatibleWith = searchParams.get('compatibleWith') ?? '';
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
    if (compatibleWith) params.set('compatibleWith', compatibleWith);
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
  }, [page, search, category, sort, compatibleWith, reloadToken]);

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
  const hasFilters = Boolean(search || category || compatibleWith);

  return (
    <div className="public-page">
      <a className="skip-link" href="#main-content">Skip to main content</a>

      {/* Top bar */}
      <header className="public-header">
        <a className="public-header__brand" href="/">
          <img src="/semrel.svg" alt="" aria-hidden="true" />
          semrel Registry
        </a>
        <div className="public-header__actions">
          <ThemeToggle />
          <a href="/api/v1/plugins" target="_blank" rel="noopener noreferrer" className="btn btn--secondary btn--header">
            API <span aria-hidden="true">↗</span><span className="sr-only">(opens in a new tab)</span>
          </a>
          {isLoggedIn
            ? <Link to="/admin" className="btn btn--primary btn--header">Admin Panel</Link>
            : <Link to="/login" className="btn btn--primary btn--header">Sign In</Link>
          }
        </div>
      </header>

      <main id="main-content" tabIndex={-1} className="public-main">
        {/* Hero */}
        <div className="public-hero">
          <h1>semrel Plugin Registry</h1>
          <p className="muted text-sm mb-2">
            Discover and install plugins for <a href="https://semrel.io" target="_blank" rel="noopener">semrel</a> — semantic versioning made simple.
          </p>
          {pagination && (
            <div className="public-hero__stats">
              <div className="public-hero__stat">
                <div className="public-hero__stat-value">{pagination.total}</div>
                <div className="muted text-xs">Plugins</div>
              </div>
            </div>
          )}
        </div>

        {/* Search + filter + sort. Every control is labelled: a placeholder is
            not a label — it disappears on typing and several screen readers
            never announce it. */}
        <search>
          <div className="flex gap-md flex-wrap mb-2">
            <div className="flex-1 search-field">
              <label className="sr-only" htmlFor="registry-search">Search plugins</label>
              <input
                id="registry-search"
                type="search"
                className="input w-full"
                placeholder="Search plugins…"
                value={searchInput}
                onChange={e => setSearchInput(e.target.value)}
              />
            </div>
            <div>
              <label className="sr-only" htmlFor="registry-category">Filter by category</label>
              <select
                id="registry-category"
                className="input select-auto"
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
                className="input select-auto"
                value={sort}
                onChange={e => updateParams({ sort: e.target.value })}
              >
                {SORTS.map(s => <option key={s.value} value={s.value}>{s.label}</option>)}
              </select>
            </div>
          </div>

          {/* "Does this work with the semrel I am running?" — the question a
              visitor actually arrives with. The catalogue carries the range per
              release; this turns it into a filter. */}
          <div className="compat-filter">
            <label htmlFor="registry-compat">Works with semrel version</label>
            <input
              id="registry-compat"
              className="input"
              type="text"
              inputMode="decimal"
              placeholder="e.g. 0.27.1"
              defaultValue={compatibleWith}
              aria-describedby="registry-compat-hint"
              onBlur={e => updateParams({ compatibleWith: e.target.value.trim() })}
              onKeyDown={e => {
                if (e.key === 'Enter') updateParams({ compatibleWith: e.currentTarget.value.trim() });
              }}
            />
            <span id="registry-compat-hint" className="field__hint">
              Plugins that declare no compatibility range are always shown.
            </span>
            {compatibleWith && (
              <button
                type="button"
                className="btn btn--secondary btn--sm"
                onClick={() => updateParams({ compatibleWith: '' })}
              >
                Clear
              </button>
            )}
          </div>
        </search>

        {/* Error — with a way out, rather than a dead end. */}
        {error && (
          <div className="alert alert--error" role="alert">
            <span>{error}</span>
            <button
              type="button"
              className="btn btn--secondary btn--sm ml-1"
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
          <div className="plugin-grid" aria-hidden="true">
            {Array.from({ length: 6 }, (_, i) => <div key={i} className="skeleton skeleton-card" />)}
          </div>
        ) : plugins.length === 0 ? (
          <div className="empty-state">
            <span className="empty-state__title">
              {hasFilters ? 'No plugins match these filters' : 'No plugins published yet'}
            </span>
            <p className="m-0 max-w-prose">
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
          <div className="plugin-grid">
            {plugins.map(p => {
              const pluginKey = p.namespace ? `${p.namespace}/${p.name}` : p.name;
              // The card is a container with a stretched link over the title
              // rather than a link wrapping everything: an interactive control
              // (the copy button) nested inside an anchor is invalid markup and
              // unreachable by keyboard in some browsers.
              return (
              <div
                key={p.id}
                className="card plugin-card flex-col gap-xs w-full"
              >
                  <div className="flex items-center justify-between gap-sm">
                    <div className="overflow-hidden">
                      {p.namespace && <span className="muted text-xs block">{p.namespace}</span>}
                      <Link
                        to={`/plugins/${encodeURIComponent(pluginKey)}`}
                        className="plugin-card__link truncate font-bold"
                      >
                        {p.name}
                      </Link>
                    </div>
                    <span className={`badge ${CAT_CLASS[p.category] ?? ''} no-shrink`}>{p.category}</span>
                  </div>
                  <p className="muted text-sm m-0 line-clamp-2">
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

                  <div className="flex items-center gap-xs flex-wrap push-bottom">
                    <span className="muted text-xs truncate min-w-0 flex-auto">by {p.author}</span>
                    <div className="flex items-center gap-xs no-shrink">
                      <span className="pill pill--success" title="Total downloads">
                        ↓ {Number(p.downloads ?? 0).toLocaleString()}
                      </span>
                      <span className="pill pill--accent" title="Total views">
                        👁 {Number(p.views ?? 0).toLocaleString()}
                      </span>
                      {p.latestVersion && (() => {
                        const ver = p.latestVersion;
                        const isDev = ver.startsWith('0.');
                        return (
                          <span className={`pill ${isDev ? 'pill--warning' : 'pill--accent'}`}>
                            <span className="sr-only">{isDev ? 'Development version ' : 'Latest version '}</span>v{ver}
                          </span>
                        );
                      })()}
                      {/* The core range the latest release declares, so the
                          answer is visible without opening the plugin. */}
                      {p.latestSemrelCore && (
                        <span className="compat-badge" title={`Requires semrel core ${p.latestSemrelCore}`}>
                          <span className="sr-only">Requires semrel core </span>
                          <span aria-hidden="true">core </span>{p.latestSemrelCore}
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {/* Pagination */}
        {pagination && pagination.pages > 1 && (
          <nav aria-label="Pagination" className="flex justify-center gap-sm">
            <button
              type="button"
              className="btn btn--secondary"
              disabled={page <= 1}
              onClick={() => updateParams({ page: String(page - 1) }, true)}
            >
              <span aria-hidden="true">←</span> Previous
            </button>
            <span className="muted text-sm line-height-2">
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
        <footer className="public-footer">
          <span className="muted text-xs">© semrel · Plugin Registry</span>
          <div className="flex gap-md flex-wrap">
            <a href="/api/v1/plugins" target="_blank" rel="noopener" className="muted text-xs">API</a>
            <a href="https://semrel.io" target="_blank" rel="noopener" className="muted text-xs">Docs</a>
            <a href="https://github.com/SemRels" target="_blank" rel="noopener" className="muted text-xs">GitHub</a>
            <a href="https://semrel.io/legal/imprint/" target="_blank" rel="noopener" className="muted text-xs">Imprint</a>
            <a href="https://semrel.io/legal/privacy/" target="_blank" rel="noopener" className="muted text-xs">Privacy</a>
            {!isLoggedIn && <Link to="/login" className="muted text-xs">Admin</Link>}
          </div>
        </footer>
      </main>
    </div>
  );
}
