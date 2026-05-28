import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { hasToken } from '../lib/api';

type Plugin = {
  id: number;
  name: string;
  description: string;
  author: string;
  category: string;
  repository: string;
  license: string;
  tags: string[];
  status: string;
};

type Version = {
  id: number;
  version: string;
  releaseDate: string | null;
  changelog: string;
  downloadUrl: string;
  prerelease: boolean;
};

const CAT_CLASS: Record<string, string> = {
  provider: 'badge--provider', analyzer: 'badge--analyzer',
  condition: 'badge--condition', hook: 'badge--hook',
  updater: 'badge--updater', generator: 'badge--generator',
};

/** Default phase hint per category for the .semrel.yaml snippet. */
const CAT_PHASE: Record<string, string> = {
  condition: 'condition',
  provider:  'provider',
  analyzer:  'analyze',
  generator: 'generate',
  updater:   'pre-tag',
  hook:      'release',
};

function configSnippet(name: string, category: string): string {
  const phase = CAT_PHASE[category] ?? 'release';
  return `plugins:
  - uses: ${name}
    phase: ${phase}`;
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  const copy = () => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    });
  };
  return (
    <button
      onClick={copy}
      style={{
        background: 'none', border: 'none', cursor: 'pointer',
        color: copied ? 'var(--success)' : 'var(--muted)',
        fontSize: 'var(--fs-xs)', padding: '2px 6px', borderRadius: 4,
        transition: 'color .15s',
      }}
      title="Copy to clipboard"
    >
      {copied ? '✓ Copied' : 'Copy'}
    </button>
  );
}

function CodeBlock({ code, label }: { code: string; label?: string }) {
  return (
    <div style={{ position: 'relative', marginTop: label ? '.5rem' : 0 }}>
      {label && (
        <div style={{ fontSize: 'var(--fs-xs)', color: 'var(--muted)', marginBottom: '.25rem', fontWeight: 600 }}>
          {label}
        </div>
      )}
      <div style={{
        background: 'var(--surface2)', borderRadius: 8, padding: '.75rem 1rem',
        fontFamily: 'monospace', fontSize: 'var(--fs-sm)', overflowX: 'auto',
        border: '1px solid var(--border)', position: 'relative',
        display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '.5rem',
      }}>
        <pre style={{ margin: 0, flex: 1, whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>{code}</pre>
        <CopyButton text={code} />
      </div>
    </div>
  );
}

export default function PluginDetailPage() {
  const { name } = useParams<{ name: string }>();
  const [plugin, setPlugin] = useState<Plugin | null>(null);
  const [versions, setVersions] = useState<Version[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const isLoggedIn = hasToken();

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
        setError('');
      })
      .catch(e => setError(e.message))
      .finally(() => setLoading(false));
  }, [name]);

  const latest = versions.find(v => !v.prerelease) ?? versions[0] ?? null;

  return (
    <div style={{ minHeight: '100vh', background: 'var(--bg)', color: 'var(--fg)' }}>
      {/* Top bar */}
      <header style={{
        borderBottom: '1px solid var(--border)', padding: '0 1.5rem',
        height: '3.25rem', display: 'flex', alignItems: 'center',
        justifyContent: 'space-between', position: 'sticky', top: 0,
        background: 'var(--bg)', zIndex: 10,
      }}>
        <Link to="/" style={{ display: 'flex', alignItems: 'center', gap: '.5rem', textDecoration: 'none', color: 'var(--fg)', fontWeight: 700 }}>
          <img src="/semrel.svg" alt="semrel" style={{ width: '1.4rem', height: '1.4rem' }} />
          semrel Registry
        </Link>
        <div style={{ display: 'flex', gap: '.5rem', alignItems: 'center' }}>
          {isLoggedIn
            ? <Link to="/admin" className="btn btn--secondary" style={{ fontSize: 'var(--fs-sm)', padding: '4px 12px' }}>Admin Panel</Link>
            : <Link to="/login" className="btn btn--primary" style={{ fontSize: 'var(--fs-sm)', padding: '4px 12px' }}>Sign In</Link>
          }
        </div>
      </header>

      <div style={{ maxWidth: '860px', margin: '0 auto', padding: '2rem 1.5rem' }}>
        {/* Breadcrumb */}
        <nav style={{ fontSize: 'var(--fs-sm)', color: 'var(--muted)', marginBottom: '1.25rem' }}>
          <Link to="/" style={{ color: 'var(--accent)' }}>Registry</Link>
          {' / '}
          <span>{name}</span>
        </nav>

        {loading && <p className="muted" style={{ textAlign: 'center', padding: '4rem 0' }}>Loading…</p>}
        {error && <div className="alert alert--error">{error} — <Link to="/">Back to registry</Link></div>}

        {plugin && (
          <>
            {/* Header */}
            <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start', flexWrap: 'wrap', marginBottom: '1.5rem' }}>
              <div style={{ flex: 1 }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '.75rem', flexWrap: 'wrap', marginBottom: '.4rem' }}>
                  <h1 style={{ margin: 0, fontSize: 'clamp(1.25rem,3vw,1.75rem)', fontWeight: 800 }}>{plugin.name}</h1>
                  <span className={`badge ${CAT_CLASS[plugin.category] ?? ''}`}>{plugin.category}</span>
                  {latest && (
                    <span style={{
                      fontSize: 'var(--fs-xs)', background: 'rgba(56,139,253,.12)',
                      color: 'var(--accent)', borderRadius: 6, padding: '2px 8px', fontFamily: 'monospace', fontWeight: 600,
                    }}>
                      v{latest.version}
                    </span>
                  )}
                </div>
                <p className="muted" style={{ margin: '0 0 .5rem', fontSize: 'var(--fs-md)' }}>
                  {plugin.description || 'No description.'}
                </p>
                <div style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', fontSize: 'var(--fs-xs)', color: 'var(--muted)' }}>
                  <span>by <strong>{plugin.author}</strong></span>
                  <span>License: <strong>{plugin.license || 'unknown'}</strong></span>
                  {plugin.repository && (
                    <a href={plugin.repository} target="_blank" rel="noopener" style={{ color: 'var(--accent)' }}>
                      GitHub ↗
                    </a>
                  )}
                </div>
              </div>
            </div>

            {/* Install */}
            <section className="card" style={{ padding: '1.25rem', marginBottom: '1.25rem' }}>
              <h2 style={{ margin: '0 0 .75rem', fontSize: 'var(--fs-md)', fontWeight: 700 }}>Installation</h2>
              <CodeBlock
                label="Install via semrel CLI"
                code={`semrel plugin install ${plugin.name}`}
              />
              <p className="muted" style={{ fontSize: 'var(--fs-xs)', marginTop: '.5rem', marginBottom: 0 }}>
                Set <code style={{ background: 'var(--surface2)', padding: '1px 4px', borderRadius: 3 }}>SEMREL_REGISTRY_URL</code> to
                point at your registry instance, or leave unset to use the default public registry.
              </p>
            </section>

            {/* Configuration */}
            <section className="card" style={{ padding: '1.25rem', marginBottom: '1.25rem' }}>
              <h2 style={{ margin: '0 0 .75rem', fontSize: 'var(--fs-md)', fontWeight: 700 }}>Configuration</h2>
              <p className="muted" style={{ fontSize: 'var(--fs-sm)', marginBottom: '.75rem' }}>
                Add this to your <code style={{ background: 'var(--surface2)', padding: '1px 4px', borderRadius: 3 }}>.semrel.yaml</code>:
              </p>
              <CodeBlock code={configSnippet(plugin.name, plugin.category)} />

              {/* Category-specific hints */}
              {plugin.category === 'provider' && (
                <p className="muted" style={{ fontSize: 'var(--fs-xs)', marginTop: '.75rem', marginBottom: 0 }}>
                  💡 Providers run in the <strong>provider</strong> phase and are responsible for reading and creating VCS tags and releases.
                  Only one provider should be active at a time.
                </p>
              )}
              {plugin.category === 'analyzer' && (
                <p className="muted" style={{ fontSize: 'var(--fs-xs)', marginTop: '.75rem', marginBottom: 0 }}>
                  💡 Analyzers run in the <strong>analyze</strong> phase and determine the next semantic version from commit messages.
                </p>
              )}
              {plugin.category === 'condition' && (
                <p className="muted" style={{ fontSize: 'var(--fs-xs)', marginTop: '.75rem', marginBottom: 0 }}>
                  💡 Conditions run first (before any git work) and can abort the release if prerequisites aren't met (e.g., wrong CI environment).
                </p>
              )}
              {plugin.category === 'generator' && (
                <p className="muted" style={{ fontSize: 'var(--fs-xs)', marginTop: '.75rem', marginBottom: 0 }}>
                  💡 Generators run in the <strong>generate</strong> phase to produce changelogs and release notes.
                  Multiple generators can run in sequence.
                </p>
              )}
              {plugin.category === 'updater' && (
                <p className="muted" style={{ fontSize: 'var(--fs-xs)', marginTop: '.75rem', marginBottom: 0 }}>
                  💡 Updaters run in the <strong>pre-tag</strong> phase to rewrite version strings in source files (e.g., package.json, go.mod).
                </p>
              )}
              {plugin.category === 'hook' && (
                <p className="muted" style={{ fontSize: 'var(--fs-xs)', marginTop: '.75rem', marginBottom: 0 }}>
                  💡 Hooks run in the <strong>release</strong> phase and can trigger notifications (Slack, Teams, email) or other post-release actions.
                </p>
              )}
            </section>

            {/* Versions */}
            <section className="card" style={{ padding: '1.25rem', marginBottom: '1.25rem' }}>
              <h2 style={{ margin: '0 0 .75rem', fontSize: 'var(--fs-md)', fontWeight: 700 }}>
                Versions <span className="muted" style={{ fontWeight: 400, fontSize: 'var(--fs-sm)' }}>({versions.length})</span>
              </h2>
              {versions.length === 0 ? (
                <p className="muted" style={{ fontSize: 'var(--fs-sm)' }}>No versions published yet.</p>
              ) : (
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 'var(--fs-sm)' }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border)' }}>
                      <th style={{ textAlign: 'left', padding: '.4rem .5rem', color: 'var(--muted)', fontWeight: 600 }}>Version</th>
                      <th style={{ textAlign: 'left', padding: '.4rem .5rem', color: 'var(--muted)', fontWeight: 600 }}>Released</th>
                      <th style={{ textAlign: 'left', padding: '.4rem .5rem', color: 'var(--muted)', fontWeight: 600 }}>Install</th>
                      <th style={{ textAlign: 'left', padding: '.4rem .5rem', color: 'var(--muted)', fontWeight: 600 }}>Assets</th>
                    </tr>
                  </thead>
                  <tbody>
                    {versions.map((v, i) => (
                      <tr key={v.id} style={{ borderBottom: '1px solid var(--border)', background: i === 0 ? 'rgba(56,139,253,.04)' : undefined }}>
                        <td style={{ padding: '.5rem', fontFamily: 'monospace', fontWeight: 600 }}>
                          v{v.version}
                          {i === 0 && !v.prerelease && (
                            <span style={{ marginLeft: '.4rem', fontSize: '10px', background: 'rgba(35,134,54,.2)', color: 'var(--success)', borderRadius: 4, padding: '1px 6px' }}>latest</span>
                          )}
                          {v.prerelease && (
                            <span style={{ marginLeft: '.4rem', fontSize: '10px', background: 'rgba(210,153,34,.2)', color: '#d7a22a', borderRadius: 4, padding: '1px 6px' }}>pre</span>
                          )}
                        </td>
                        <td style={{ padding: '.5rem', color: 'var(--muted)' }}>
                          {v.releaseDate ? new Date(v.releaseDate).toLocaleDateString() : '—'}
                        </td>
                        <td style={{ padding: '.5rem' }}>
                          <code style={{ background: 'var(--surface2)', padding: '2px 6px', borderRadius: 4, fontSize: 'var(--fs-xs)' }}>
                            semrel plugin install {plugin.name}@{v.version}
                          </code>
                        </td>
                        <td style={{ padding: '.5rem' }}>
                          {v.downloadUrl ? (
                            <a href={v.downloadUrl} target="_blank" rel="noopener" style={{ color: 'var(--accent)', fontSize: 'var(--fs-xs)' }}>
                              Download ↗
                            </a>
                          ) : '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </section>

            {/* Changelog of latest */}
            {latest?.changelog && (
              <section className="card" style={{ padding: '1.25rem', marginBottom: '1.25rem' }}>
                <h2 style={{ margin: '0 0 .75rem', fontSize: 'var(--fs-md)', fontWeight: 700 }}>
                  Release notes <span className="muted" style={{ fontWeight: 400, fontSize: 'var(--fs-sm)' }}>v{latest.version}</span>
                </h2>
                <pre style={{ margin: 0, fontSize: 'var(--fs-sm)', whiteSpace: 'pre-wrap', wordBreak: 'break-word', color: 'var(--fg-muted)' }}>
                  {latest.changelog}
                </pre>
              </section>
            )}

            {/* Tags */}
            {plugin.tags?.length > 0 && (
              <div style={{ display: 'flex', gap: '.35rem', flexWrap: 'wrap' }}>
                {plugin.tags.map(t => (
                  <span key={t} style={{ fontSize: 'var(--fs-xs)', background: 'rgba(56,139,253,.1)', color: 'var(--accent)', borderRadius: 5, padding: '2px 8px' }}>{t}</span>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
