import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getPlugin, listVersions, createVersion, deleteVersion } from '../lib/api';
import type { Plugin, PluginVersion } from '../lib/api';
import { marked } from 'marked';

export default function VersionsPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [plugin, setPlugin]   = useState<Plugin | null>(null);
  const [versions, setVersions] = useState<PluginVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError]     = useState('');
  const [showForm, setShowForm] = useState(false);
  const [expandedId, setExpandedId] = useState<number | null>(null);
  const [form, setForm]       = useState({ version:'', releaseDate:'', downloadUrl:'', changelog:'', prerelease:false, checksums:'{}' });
  const [saving, setSaving]   = useState(false);
  const [formError, setFormError] = useState('');

  useEffect(() => {
    if (!id) return;
    Promise.all([getPlugin(id), listVersions(id)])
      .then(([p, v]) => { setPlugin(p.data); setVersions(v.data ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : 'Failed'); setLoading(false); });
  }, [id]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault(); setSaving(true); setFormError('');
    let checksums: Record<string, string> = {};
    try { checksums = JSON.parse(form.checksums) as Record<string, string>; }
    catch { setFormError('Checksums must be valid JSON.'); setSaving(false); return; }
    try {
      const { data: v } = await createVersion(id!, { version:form.version, releaseDate:form.releaseDate||undefined, downloadUrl:form.downloadUrl, changelog:form.changelog, prerelease:form.prerelease, checksums });
      setVersions(p => [v, ...p]);
      setShowForm(false);
      setForm({ version:'', releaseDate:'', downloadUrl:'', changelog:'', prerelease:false, checksums:'{}' });
    } catch (e: unknown) { setFormError(e instanceof Error ? e.message : 'Failed'); }
    finally { setSaving(false); }
  }

  async function handleDelete(v: PluginVersion) {
    if (!globalThis.confirm(`Retract version v${v.version}? This cannot be undone.`)) return;
    try {
      await deleteVersion(id!, v.id);
      setVersions(prev => prev.filter(x => x.id !== v.id));
    } catch (e: unknown) { alert(e instanceof Error ? e.message : 'Delete failed'); }
  }

  if (loading) return <div className="page__body muted">Loading…</div>;

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">Versions · <span style={{ color:'var(--accent)' }}>{plugin?.name}</span></h1>
        <div className="flex gap-sm">
          <button type="button" className="btn btn--sm" onClick={() => navigate('/plugins')}>← Back</button>
          <button type="button" className="btn btn--primary btn--sm" onClick={() => setShowForm(s => !s)}>
            {showForm ? 'Cancel' : '+ Add Version'}
          </button>
        </div>
      </div>
      <div className="page__body">
        {error && <div className="alert alert--error">{error}</div>}

        {showForm && (
          <div className="card" style={{ maxWidth:600, marginBottom:'1rem' }}>
            <h2 style={{ marginBottom:'.75rem' }}>New Version</h2>
            {formError && <div className="alert alert--error">{formError}</div>}
            <form onSubmit={(e) => { void handleCreate(e); }}>
              <div style={{ display:'grid', gridTemplateColumns:'1fr 1fr', gap:'.75rem' }}>
                <div className="field"><label htmlFor="ver">Version *</label>
                  <input id="ver" className="input" value={form.version} onChange={e => setForm(f=>({...f,version:e.target.value}))} placeholder="0.1.0" required /></div>
                <div className="field"><label htmlFor="date">Release date</label>
                  <input id="date" type="datetime-local" className="input" value={form.releaseDate} onChange={e => setForm(f=>({...f,releaseDate:e.target.value}))} /></div>
              </div>
              <div className="field"><label htmlFor="url">Download URL *</label>
                <input id="url" type="url" className="input" value={form.downloadUrl} onChange={e => setForm(f=>({...f,downloadUrl:e.target.value}))} placeholder="https://github.com/…" required /></div>
              <div className="field"><label htmlFor="cl">Changelog</label>
                <textarea id="cl" className="textarea" value={form.changelog} onChange={e => setForm(f=>({...f,changelog:e.target.value}))} /></div>
              <div className="field"><label htmlFor="cs">Checksums (JSON)</label>
                <textarea id="cs" className="textarea" style={{ fontFamily:'monospace', fontSize:'var(--fs-xs)' }} value={form.checksums} onChange={e => setForm(f=>({...f,checksums:e.target.value}))} placeholder={'{"linux_amd64":"<sha256>"}'} /></div>
              <label style={{ display:'flex', alignItems:'center', gap:'.375rem', fontSize:'var(--fs-sm)', marginBottom:'.75rem', cursor:'pointer' }}>
                <input type="checkbox" checked={form.prerelease} onChange={e => setForm(f=>({...f,prerelease:e.target.checked}))} />
                {' '}Pre-release
              </label>
              <button type="submit" className="btn btn--primary" disabled={saving}>{saving ? 'Saving…' : 'Publish'}</button>
            </form>
          </div>
        )}

        {versions.length === 0 ? (
          <div className="muted" style={{ padding:'2rem 0', textAlign:'center' }}>No versions yet.</div>
        ) : (
          <div className="table-wrap">
            <table className="table--stack">
              <thead><tr><th>Version</th><th>Released</th><th>Channel</th><th>Views</th><th>Downloads</th><th>Download URL</th><th>Platforms</th><th></th></tr></thead>
              <tbody>
                {versions.map(v => (
                  <>
                    <tr key={v.id}>
                      <td data-label="Version">
                        <button
                          type="button"
                          onClick={() => setExpandedId(expandedId === v.id ? null : v.id)}
                          style={{ background:'none', border:'none', cursor:'pointer', color:'var(--accent)', fontFamily:'monospace', padding:0, fontSize:'inherit' }}
                          title={v.changelog ? 'Click to view release notes' : 'No release notes'}
                        >
                          {expandedId === v.id ? '▾' : '▸'} v{v.version}
                        </button>
                      </td>
                      <td data-label="Released" className="muted" style={{ fontSize:'var(--fs-sm)' }}>{v.releaseDate ? new Intl.DateTimeFormat('en',{dateStyle:'medium'}).format(new Date(v.releaseDate)) : '—'}</td>
                      <td data-label="Channel">{v.prerelease
                        ? <span className="badge" style={{ background:'rgba(210,153,34,.15)',color:'var(--warning)',borderColor:'rgba(210,153,34,.3)' }}>pre</span>
                        : <span className="badge" style={{ background:'rgba(63,185,80,.12)',color:'var(--success)',borderColor:'rgba(63,185,80,.25)' }}>stable</span>
                      }</td>
                      <td data-label="Views" style={{ fontSize:'var(--fs-sm)' }}>{Number(v.views ?? 0).toLocaleString()}</td>
                      <td data-label="Downloads" style={{ fontSize:'var(--fs-sm)' }}>{Number(v.downloads ?? 0).toLocaleString()}</td>
                      <td data-label="Download" style={{ fontSize:'var(--fs-xs)', maxWidth:200 }} className="muted truncate">
                        <a
                          href={`/api/v1/plugins/${encodeURIComponent(id ?? '')}/versions/${encodeURIComponent(v.version)}/download`}
                          target="_blank"
                          rel="noopener"
                        >
                          Download via registry
                        </a>
                      </td>
                      <td data-label="Platforms" className="muted" style={{ fontSize:'var(--fs-sm)' }}>{v.checksums ? Object.keys(v.checksums).length : 0}</td>
                      <td data-label="Actions">
                        <button
                          type="button"
                          className="btn btn--sm btn--danger"
                          onClick={() => { void handleDelete(v); }}
                          title="Retract version"
                        >
                          Retract
                        </button>
                      </td>
                    </tr>
                    {expandedId === v.id && (
                      <tr key={`${v.id}-notes`}>
                        <td colSpan={8} style={{ background:'rgba(255,255,255,.03)', padding:'1rem 1.25rem', borderTop:'1px solid var(--border)' }}>
                          {v.changelog ? (
                            <div
                              className="prose"
                              style={{ fontSize:'var(--fs-sm)', color:'var(--fg)', maxWidth:'64rem' }}
                              dangerouslySetInnerHTML={{ __html: marked.parse(v.changelog) as string }}
                            />
                          ) : (
                            <span className="muted" style={{ fontSize:'var(--fs-sm)' }}>No release notes for this version.</span>
                          )}
                        </td>
                      </tr>
                    )}
                  </>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  );
}


export default function VersionsPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [plugin, setPlugin]   = useState<Plugin | null>(null);
  const [versions, setVersions] = useState<PluginVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError]     = useState('');
  const [showForm, setShowForm] = useState(false);
  const [form, setForm]       = useState({ version:'', releaseDate:'', downloadUrl:'', changelog:'', prerelease:false, checksums:'{}' });
  const [saving, setSaving]   = useState(false);
  const [formError, setFormError] = useState('');

  useEffect(() => {
    if (!id) return;
    Promise.all([getPlugin(id), listVersions(id)])
      .then(([p, v]) => { setPlugin(p.data); setVersions(v.data ?? []); setLoading(false); })
      .catch((e: unknown) => { setError(e instanceof Error ? e.message : 'Failed'); setLoading(false); });
  }, [id]);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault(); setSaving(true); setFormError('');
    let checksums: Record<string, string> = {};
    try { checksums = JSON.parse(form.checksums) as Record<string, string>; }
    catch { setFormError('Checksums must be valid JSON.'); setSaving(false); return; }
    try {
      const { data: v } = await createVersion(id!, { version:form.version, releaseDate:form.releaseDate||undefined, downloadUrl:form.downloadUrl, changelog:form.changelog, prerelease:form.prerelease, checksums });
      setVersions(p => [v, ...p]);
      setShowForm(false);
      setForm({ version:'', releaseDate:'', downloadUrl:'', changelog:'', prerelease:false, checksums:'{}' });
    } catch (e: unknown) { setFormError(e instanceof Error ? e.message : 'Failed'); }
    finally { setSaving(false); }
  }

  if (loading) return <div className="page__body muted">Loading…</div>;

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">Versions · <span style={{ color:'var(--accent)' }}>{plugin?.name}</span></h1>
        <div className="flex gap-sm">
          <button type="button" className="btn btn--sm" onClick={() => navigate('/plugins')}>← Back</button>
          <button type="button" className="btn btn--primary btn--sm" onClick={() => setShowForm(s => !s)}>
            {showForm ? 'Cancel' : '+ Add Version'}
          </button>
        </div>
      </div>
      <div className="page__body">
        {error && <div className="alert alert--error">{error}</div>}

        {showForm && (
          <div className="card" style={{ maxWidth:600, marginBottom:'1rem' }}>
            <h2 style={{ marginBottom:'.75rem' }}>New Version</h2>
            {formError && <div className="alert alert--error">{formError}</div>}
            <form onSubmit={(e) => { void handleCreate(e); }}>
              <div style={{ display:'grid', gridTemplateColumns:'1fr 1fr', gap:'.75rem' }}>
                <div className="field"><label htmlFor="ver">Version *</label>
                  <input id="ver" className="input" value={form.version} onChange={e => setForm(f=>({...f,version:e.target.value}))} placeholder="0.1.0" required /></div>
                <div className="field"><label htmlFor="date">Release date</label>
                  <input id="date" type="datetime-local" className="input" value={form.releaseDate} onChange={e => setForm(f=>({...f,releaseDate:e.target.value}))} /></div>
              </div>
              <div className="field"><label htmlFor="url">Download URL *</label>
                <input id="url" type="url" className="input" value={form.downloadUrl} onChange={e => setForm(f=>({...f,downloadUrl:e.target.value}))} placeholder="https://github.com/…" required /></div>
              <div className="field"><label htmlFor="cl">Changelog</label>
                <textarea id="cl" className="textarea" value={form.changelog} onChange={e => setForm(f=>({...f,changelog:e.target.value}))} /></div>
              <div className="field"><label htmlFor="cs">Checksums (JSON)</label>
                <textarea id="cs" className="textarea" style={{ fontFamily:'monospace', fontSize:'var(--fs-xs)' }} value={form.checksums} onChange={e => setForm(f=>({...f,checksums:e.target.value}))} placeholder={'{"linux_amd64":"<sha256>"}'} /></div>
              <label style={{ display:'flex', alignItems:'center', gap:'.375rem', fontSize:'var(--fs-sm)', marginBottom:'.75rem', cursor:'pointer' }}>
                <input type="checkbox" checked={form.prerelease} onChange={e => setForm(f=>({...f,prerelease:e.target.checked}))} />
                {' '}Pre-release
              </label>
              <button type="submit" className="btn btn--primary" disabled={saving}>{saving ? 'Saving…' : 'Publish'}</button>
            </form>
          </div>
        )}

        {versions.length === 0 ? (
          <div className="muted" style={{ padding:'2rem 0', textAlign:'center' }}>No versions yet.</div>
        ) : (
          <div className="table-wrap">
            <table className="table--stack">
              <thead><tr><th>Version</th><th>Released</th><th>Channel</th><th>Views</th><th>Downloads</th><th>Download URL</th><th>Platforms</th></tr></thead>
              <tbody>
                {versions.map(v => (
                  <tr key={v.id}>
                    <td data-label="Version"><code>v{v.version}</code></td>
                    <td data-label="Released" className="muted" style={{ fontSize:'var(--fs-sm)' }}>{v.releaseDate ? new Intl.DateTimeFormat('en',{dateStyle:'medium'}).format(new Date(v.releaseDate)) : '—'}</td>
                    <td data-label="Channel">{v.prerelease
                      ? <span className="badge" style={{ background:'rgba(210,153,34,.15)',color:'var(--warning)',borderColor:'rgba(210,153,34,.3)' }}>pre</span>
                      : <span className="badge" style={{ background:'rgba(63,185,80,.12)',color:'var(--success)',borderColor:'rgba(63,185,80,.25)' }}>stable</span>
                    }</td>
                    <td data-label="Views" style={{ fontSize:'var(--fs-sm)' }}>{Number(v.views ?? 0).toLocaleString()}</td>
                    <td data-label="Downloads" style={{ fontSize:'var(--fs-sm)' }}>{Number(v.downloads ?? 0).toLocaleString()}</td>
                    <td data-label="Download" style={{ fontSize:'var(--fs-xs)', maxWidth:200 }} className="muted truncate">
                      <a
                        href={`/api/v1/plugins/${encodeURIComponent(id ?? '')}/versions/${encodeURIComponent(v.version)}/download`}
                        target="_blank"
                        rel="noopener"
                      >
                        Download via registry
                      </a>
                    </td>
                    <td data-label="Platforms" className="muted" style={{ fontSize:'var(--fs-sm)' }}>{v.checksums ? Object.keys(v.checksums).length : 0}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </>
  );
}
