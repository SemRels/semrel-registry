import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import Markdown from '../components/Markdown';
import CopyButton from '../components/CopyButton';
import ReadmeSection from '../components/ReadmeSection';
import { revalidatePlugin } from '../lib/api';
import { useCurrentUser } from '../hooks/useCurrentUser';
import type { ValidationResult } from '../lib/api';

/** Creates or updates a <meta> tag in the document head. */
function setOrCreate(selector: string, attr: string, attrValue: string, content: string) {
  let el = document.querySelector<HTMLMetaElement>(selector);
  if (!el) {
    el = document.createElement('meta');
    el.setAttribute(attr, attrValue);
    document.head.appendChild(el);
  }
  el.setAttribute('content', content);
}

function ValidationPanel({ checks, summary, valid, validatedAt }: ValidationResult & { validatedAt?: string }) {
  return (
    <div className="validation-panel">
      <div className="flex items-center justify-between mb-1 flex-wrap gap-xs">
        <div className={`inline-flex items-center gap-xs text-xs font-bold ${valid ? 'validation-panel__status--pass' : 'validation-panel__status--fail'}`}>
          {valid ? '✓ All checks passed' : '✗ Some checks failed'}
          {summary && <span className="validation-panel__summary"> — {summary}</span>}
        </div>
        {validatedAt && (
          <span className="text-xs muted">
            checked {new Date(validatedAt).toLocaleString()}
          </span>
        )}
      </div>
      <div className="validation-panel__grid">
        {checks.map(ch => (
          <div key={ch.id} className="validation-panel__check">
            <span className={ch.passed ? 'validation-panel__icon--pass' : 'validation-panel__icon--fail'}>
              {ch.passed ? '✓' : '✗'}
            </span>
            <span className={ch.passed ? '' : 'muted'}>
              {ch.label}
              {ch.message && <span className="validation-panel__message">{ch.message}</span>}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

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
  status: string;
  views?: number;
  downloads?: number;
  validationChecks?: ValidationResult;
  validatedAt?: string;
};

type Version = {
  id: number;
  version: string;
  releaseDate: string | null;
  changelog: string;
  downloadUrl: string;
  downloadUrls?: Record<string, string>;
  checksums?: Record<string, string>;
  prerelease: boolean;
  compatibility?: { semrelCore?: string };
  yanked?: boolean;
  yankedReason?: string;
  views?: number;
  downloads?: number;
};

const CAT_CLASS: Record<string, string> = {
  provider: 'badge--provider', analyzer: 'badge--analyzer',
  condition: 'badge--condition', hook: 'badge--hook',
  updater: 'badge--updater', generator: 'badge--generator',
  packager: 'badge--packager', publisher: 'badge--publisher',
};

const CAT_PHASE: Record<string, string> = {
  condition: 'condition',
  provider:  'provider',
  analyzer:  'analyze',
  generator: 'generate',
  updater:   'pre-tag',
  packager:  'package',
  publisher: 'publish',
  hook:      'release',
};

const PLATFORM_LABELS: Record<string, string> = {
  linux_amd64:   'Linux x64',
  linux_arm64:   'Linux ARM64',
  darwin_amd64:  'macOS Intel',
  darwin_arm64:  'macOS Apple Silicon',
  windows_amd64: 'Windows x64',
  windows_arm64: 'Windows ARM64',
};

const PLATFORM_ORDER = ['linux_amd64', 'linux_arm64', 'darwin_amd64', 'darwin_arm64', 'windows_amd64', 'windows_arm64'];

/** Returns true for development versions (semver major == 0). */
function isDevVersion(version: string): boolean {
  return version.startsWith('0.');
}

function configSnippet(namespace: string | undefined, name: string, category: string): string {
  const phase = CAT_PHASE[category] ?? 'release';
  const ref = namespace ? `${namespace}/${name}` : name;
  return `plugins:
  - uses: ${ref}
    phase: ${phase}`;
}

function CodeBlock({ code, label }: { code: string; label?: string }) {
  return (
    <div className={label ? 'mt-1' : ''}>
      {label && <div className="text-xs muted mb-1 font-semibold">{label}</div>}
      <div className="code-block">
        <pre className="code-block__pre">{code}</pre>
        <CopyButton text={code} label={label ? `Copy ${label.toLowerCase()}` : 'Copy'} showLabel />
      </div>
    </div>
  );
}

/** Renders registry-supplied markdown through the sanitising renderer. */
function MarkdownContent({ md }: { md: string }) {
  return <Markdown source={md} className="markdown-content--relaxed" />;
}

/** Version badge reflecting semver semantics. */
function VersionBadge({ version, isLatest }: { version: string; isLatest: boolean }) {
  const dev = isDevVersion(version);
  if (dev) {
    return <span className="pill pill--warning ml-1">dev</span>;
  }
  if (isLatest) {
    return <span className="pill pill--success ml-1">latest</span>;
  }
  return null;
}

/** Expandable multi-arch download links for a version. */
function DownloadLinks({ downloadUrls }: { downloadUrls?: Record<string, string> }) {
  const [open, setOpen] = useState(false);
  if (!downloadUrls || Object.keys(downloadUrls).length === 0) return <span className="muted">—</span>;

  const platforms = PLATFORM_ORDER.filter(k => downloadUrls[k]);

  return (
    <div>
      <button
        onClick={() => setOpen(o => !o)}
        className="download-links__toggle"
      >
        {open ? '▾ Hide' : `▸ Download (${platforms.length} platforms)`}
      </button>
      {open && (
        <div className="download-links__list">
          {platforms.map(key => (
            <a
              key={key}
              href={downloadUrls[key]}
              target="_blank"
              rel="noopener"
              className="download-links__item"
            >
              <span className="download-links__platform">{key}</span>
              {PLATFORM_LABELS[key] ?? key}
              {key.startsWith('windows') ? ' (.exe)' : ''}
              <span className="download-links__external">↗</span>
            </a>
          ))}
        </div>
      )}
    </div>
  );
}

export default function PluginDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [plugin, setPlugin] = useState<Plugin | null>(null);
  const [versions, setVersions] = useState<Version[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [expandedVersionId, setExpandedVersionId] = useState<number | null>(null);
  const [checks, setChecks] = useState<ValidationResult | null>(null);
  const [revalidating, setRevalidating] = useState(false);
  const { user } = useCurrentUser();
  const isLoggedIn = user !== null;

  useEffect(() => {
    if (!name) return;
    setLoading(true);
    Promise.all([
      fetch(`/api/v1/plugins/${name}`).then(r => {
        if (!r.ok) throw new Error('Plugin not found');
        return r.json();
      }).then(d => d.data ?? d),
      fetch(`/api/v1/plugins/${name}/versions?limit=50`).then(r => r.json()),
    ])
      .then(([p, v]) => {
        setPlugin(p);
        setVersions(v.data ?? []);
        setChecks(p.validationChecks ?? null);
        setError('');
      })
      .catch(e => setError(e.message))
      .finally(() => setLoading(false));
  }, [name]);

  async function handleRevalidate() {
    if (!plugin) return;
    setRevalidating(true);
    try {
      const result = await revalidatePlugin(plugin.id);
      setChecks(result);
    } catch { /* silent */ }
    finally { setRevalidating(false); }
  }

  // Update document meta tags for social sharing and SEO
  useEffect(() => {
    if (!plugin) return;
    const pluginRef = plugin.namespace ? `${plugin.namespace}/${plugin.name}` : plugin.name;
    const title = `${pluginRef} — semrel Registry`;
    const desc  = plugin.description
      ? `${plugin.description} | semrel plugin for ${plugin.category} — install with: semrel plugin install ${pluginRef}`
      : `semrel ${plugin.category} plugin: ${pluginRef}. Install with semrel plugin install ${pluginRef}.`;

    document.title = title;
    setOrCreate('meta[name="description"]',       'name',     'description',       desc);
    setOrCreate('meta[property="og:title"]',      'property', 'og:title',          title);
    setOrCreate('meta[property="og:description"]','property', 'og:description',    desc);
    setOrCreate('meta[property="og:url"]',        'property', 'og:url',            globalThis.location.href);
    setOrCreate('meta[name="twitter:title"]',     'name',     'twitter:title',     title);
    setOrCreate('meta[name="twitter:description"]','name',    'twitter:description',desc);

    return () => {
      document.title = 'semrel Registry — Discover & Install Semantic Release Plugins';
    };
  }, [plugin]);

  const latest = versions.find(v => !v.prerelease) ?? versions[0] ?? null;

  return (
    <div className="public-page">
      {/* Top bar */}
      <header className="public-header">
        <Link to="/" className="public-header__brand">
          <img src="/semrel.svg" alt="semrel" />
          semrel Registry
        </Link>
        <div className="public-header__actions">
          {isLoggedIn
            ? <Link to="/admin" className="btn btn--secondary btn--header">Admin Panel</Link>
            : <Link to="/login" className="btn btn--primary btn--header">Sign In</Link>
          }
        </div>
      </header>

      <div className="public-main public-main--narrow">
        {/* Breadcrumb */}
        <nav className="public-breadcrumb">
          <Link to="/">Registry</Link>
          {' / '}
          <span>{name}</span>
        </nav>

        {loading && <p className="muted text-center public-loading">Loading…</p>}
        {error && <div className="alert alert--error">{error} — <Link to="/">Back to registry</Link></div>}

        {plugin && (
          <>
            {/* Header */}
            <div className="flex gap-md items-start flex-wrap mb-3">
              <div className="flex-1">
                <div className="flex items-center gap-sm flex-wrap mb-1">
                  <h1 className="plugin-detail__title">
                    {plugin.namespace && <span className="plugin-detail__namespace">{plugin.namespace}/</span>}
                    {plugin.name}
                  </h1>
                  <span className={`badge ${CAT_CLASS[plugin.category] ?? ''}`}>{plugin.category}</span>
                  {latest && (
                    <span className={`pill ${isDevVersion(latest.version) ? 'pill--warning' : 'pill--accent'} pill--roomy`}>
                      v{latest.version}
                      {isDevVersion(latest.version) && <span className="pill__sub">dev</span>}
                    </span>
                  )}
                  <span className="pill pill--success pill--roomy" title="Total downloads">
                    ↓ {Number(plugin.downloads ?? 0).toLocaleString()}
                  </span>
                  <span className="pill pill--accent pill--roomy" title="Total views">
                    👁 {Number(plugin.views ?? 0).toLocaleString()}
                  </span>
                </div>
                <p className="muted mb-1 text-md">
                  {plugin.description || 'No description.'}
                </p>
                <div className="flex gap-md flex-wrap text-xs muted">
                  <span>by <strong>{plugin.author}</strong></span>
                  <span>License: <strong>{plugin.license || 'unknown'}</strong></span>
                  {plugin.repository && (
                    <a href={plugin.repository} target="_blank" rel="noopener">
                      GitHub ↗
                    </a>
                  )}
                </div>
              </div>
            </div>

            {/* Install */}
            <section className="card section-card">
              <h2 className="section-card__title">Installation</h2>
              <CodeBlock
                label="Install via semrel CLI"
                code={`semrel plugin install ${plugin.namespace ? `${plugin.namespace}/${plugin.name}` : plugin.name}`}
              />
              <p className="muted text-xs mt-1 m-0">
                Set <code className="inline-code">SEMREL_REGISTRY_URL</code> to
                point at your registry instance, or leave unset to use the default public registry.
              </p>
            </section>

            {/* Quality / MVP Standards */}
            <section className="card section-card">
              <div className="flex items-center justify-between gap-sm flex-wrap">
                <h2 className="section-card__title m-0">
                  Quality Standards
                  {checks && (
                    <span className={`quality-badge ${checks.valid ? 'quality-badge--pass' : 'quality-badge--fail'}`}>
                      {checks.valid ? '✓ MVP' : '✗ MVP'}
                    </span>
                  )}
                </h2>
                {isLoggedIn && (
                  <button
                    type="button"
                    className="btn btn--sm"
                    onClick={() => { void handleRevalidate(); }}
                    disabled={revalidating}
                    title="Re-run quality checks"
                  >
                    {revalidating ? '⏳ Checking…' : checks ? '↻ Re-check' : '▶ Run check'}
                  </button>
                )}
              </div>
              {checks
                ? <ValidationPanel {...checks} validatedAt={plugin.validatedAt} />
                : <p className="muted text-sm m-0 mt-1">
                    No quality checks run yet.{isLoggedIn ? ' Click "Run check" to validate.' : ''}
                  </p>
              }
            </section>

            {/* README — the plugin's own documentation. Until now the only
                description on this page was the one-line catalogue summary. */}
            <ReadmeSection pluginId={plugin.id} repository={plugin.repository} />

            {/* Configuration */}
            <section className="card section-card">
              <h2 className="section-card__title">Configuration</h2>
              <p className="muted text-sm mb-1">
                Add this to your <code className="inline-code">.semrel.yaml</code>:
              </p>
              <CodeBlock code={configSnippet(plugin.namespace, plugin.name, plugin.category)} />

              {plugin.category === 'provider' && (
                <p className="muted text-xs mt-2 m-0">
                  💡 Providers run in the <strong>provider</strong> phase and are responsible for reading and creating VCS tags and releases.
                  Only one provider should be active at a time.
                </p>
              )}
              {plugin.category === 'analyzer' && (
                <p className="muted text-xs mt-2 m-0">
                  💡 Analyzers run in the <strong>analyze</strong> phase and determine the next semantic version from commit messages.
                </p>
              )}
              {plugin.category === 'condition' && (
                <p className="muted text-xs mt-2 m-0">
                  💡 Conditions run first (before any git work) and can abort the release if prerequisites aren't met (e.g., wrong CI environment).
                </p>
              )}
              {plugin.category === 'generator' && (
                <p className="muted text-xs mt-2 m-0">
                  💡 Generators run in the <strong>generate</strong> phase to produce changelogs and release notes.
                  Multiple generators can run in sequence.
                </p>
              )}
              {plugin.category === 'updater' && (
                <p className="muted text-xs mt-2 m-0">
                  💡 Updaters run in the <strong>pre-tag</strong> phase to rewrite version strings in source files (e.g., package.json, go.mod).
                </p>
              )}
              {plugin.category === 'hook' && (
                <p className="muted text-xs mt-2 m-0">
                  💡 Hooks run in the <strong>release</strong> phase and can trigger notifications (Slack, Teams, email) or other post-release actions.
                </p>
              )}
            </section>

            {/* Versions */}
            <section className="card section-card">
              <h2 className="section-card__title">
                Versions <span className="muted count-suffix">({versions.length})</span>
              </h2>
              {versions.length === 0 ? (
                <p className="muted text-sm">No versions published yet.</p>
              ) : (
                <div className="table-wrap">
                  <table className="table--stack plugin-versions-table">
                    <thead>
                      <tr>
                        <th>Version</th>
                        <th>Released</th>
                        <th>Core</th>
                        <th>Install</th>
                        <th>Stats</th>
                        <th>Downloads</th>
                      </tr>
                    </thead>
                    <tbody>
                      {versions.map((v, i) => (
                        <>
                          <tr
                            key={v.id}
                            className={[
                              i === 0 ? 'plugin-versions-table__row--latest' : '',
                              expandedVersionId === v.id ? 'plugin-versions-table__row--expanded' : '',
                            ].filter(Boolean).join(' ')}
                          >
                            <td data-label="Version">
                              <button
                                type="button"
                                onClick={() => setExpandedVersionId(expandedVersionId === v.id ? null : v.id)}
                                className={`plugin-versions-table__toggle ${v.changelog ? '' : 'plugin-versions-table__toggle--empty'}`}
                                title={v.changelog ? 'Click to view release notes' : 'No release notes'}
                              >
                                {v.changelog ? (expandedVersionId === v.id ? '▾' : '▸') : <span className="plugin-versions-table__dash">—</span>}
                                {' '}v{v.version}
                              </button>
                              {v.yanked
                                ? <span className="pill pill--danger ml-1">yanked</span>
                                : v.prerelease
                                  ? <span className="pill pill--warning ml-1">pre</span>
                                  : <VersionBadge version={v.version} isLatest={i === 0} />
                              }
                              {/* The reason matters more than the badge: it is
                                  what tells someone on this version whether to
                                  move urgently or at leisure. */}
                              {v.yanked && v.yankedReason && (
                                <p className="field__error plugin-versions-table__yank-reason">
                                  {v.yankedReason}
                                </p>
                              )}
                            </td>
                            <td data-label="Released" className="muted">
                              {v.releaseDate ? new Date(v.releaseDate).toLocaleDateString() : '—'}
                            </td>
                            <td data-label="Core" className="muted mono text-xs">
                              {v.compatibility?.semrelCore || '—'}
                            </td>
                            <td data-label="Install">
                              <code className="inline-code text-xs">
                                semrel plugin install {plugin.namespace ? `${plugin.namespace}/${plugin.name}` : plugin.name}@{v.version}
                              </code>
                            </td>
                            <td data-label="Stats">
                              <div className="flex gap-xs flex-wrap">
                                <span title="Downloads" className="pill pill--success">
                                  ↓ {Number(v.downloads ?? 0).toLocaleString()}
                                </span>
                                <span title="Views" className="pill pill--accent">
                                  👁 {Number(v.views ?? 0).toLocaleString()}
                                </span>
                              </div>
                            </td>
                            <td data-label="Downloads">
                              <DownloadLinks downloadUrls={v.downloadUrls} />
                            </td>
                          </tr>
                          {expandedVersionId === v.id && v.changelog && (
                            <tr key={`${v.id}-notes`}>
                              <td colSpan={5} className="plugin-versions-table__notes">
                                <MarkdownContent md={v.changelog} />
                              </td>
                            </tr>
                          )}
                        </>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </section>

            {/* Latest release notes (kept for quick access on page load) */}
            {latest?.changelog && expandedVersionId === null && (
              <section className="card section-card">
                <h2 className="section-card__title">
                  Release notes <span className="muted count-suffix">v{latest.version}</span>
                </h2>
                <MarkdownContent md={latest.changelog} />
              </section>
            )}

            {/* Tags */}
            {plugin.tags?.length > 0 && (
              <div className="flex gap-xs flex-wrap">
                {plugin.tags.map(t => (
                  <span key={t} className="pill pill--accent pill--roomy">{t}</span>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
