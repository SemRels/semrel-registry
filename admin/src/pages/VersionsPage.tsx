import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getPlugin, listVersions, createVersion } from '../lib/api';
import type { Plugin, PluginVersion } from '../lib/api';

export default function VersionsPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const [plugin, setPlugin] = useState<Plugin | null>(null);
  const [versions, setVersions] = useState<PluginVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [showForm, setShowForm] = useState(false);

  const [form, setForm] = useState({
    version: '',
    releaseDate: '',
    downloadUrl: '',
    changelog: '',
    prerelease: false,
    checksums: '{}',
  });
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState('');

  useEffect(() => {
    if (!id) return;
    Promise.all([getPlugin(id), listVersions(id)])
      .then(([p, v]) => {
        setPlugin(p.data);
        setVersions(v.data ?? []);
        setLoading(false);
      })
      .catch((e: unknown) => {
        setError(e instanceof Error ? e.message : 'Failed to load');
        setLoading(false);
      });
  }, [id]);

  async function handleCreateVersion(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setFormError('');

    let checksums: Record<string, string> = {};
    try {
      checksums = JSON.parse(form.checksums) as Record<string, string>;
    } catch {
      setFormError('Checksums must be valid JSON object.');
      setSaving(false);
      return;
    }

    try {
      const { data: v } = await createVersion(id!, {
        version: form.version,
        releaseDate: form.releaseDate || undefined,
        downloadUrl: form.downloadUrl,
        changelog: form.changelog,
        prerelease: form.prerelease,
        checksums,
      });
      setVersions((prev) => [v, ...prev]);
      setShowForm(false);
      setForm({ version: '', releaseDate: '', downloadUrl: '', changelog: '', prerelease: false, checksums: '{}' });
    } catch (e: unknown) {
      setFormError(e instanceof Error ? e.message : 'Save failed');
    } finally {
      setSaving(false);
    }
  }

  if (loading) return <div className="loading-block"><span className="spinner" /> Loading…</div>;

  return (
    <>
      <button
        type="button"
        className="btn btn-ghost btn-sm"
        style={{ marginBottom: '1rem' }}
        onClick={() => navigate('/plugins')}
      >
        ← Back to plugins
      </button>

      <div className="flex-between" style={{ marginBottom: '0.25rem' }}>
        <h1 className="page-title">
          Versions · <span style={{ color: 'var(--accent-strong)' }}>{plugin?.name}</span>
        </h1>
        <button
          type="button"
          className="btn btn-primary"
          onClick={() => setShowForm((s) => !s)}
        >
          {showForm ? 'Cancel' : '+ Add Version'}
        </button>
      </div>
      <p className="page-subtitle">{plugin?.description}</p>

      {error && <div className="alert alert-error">{error}</div>}

      {showForm && (
        <div className="card" style={{ marginBottom: '1.5rem', maxWidth: 640 }}>
          <h2 style={{ margin: '0 0 1rem', fontSize: '1rem' }}>New Version</h2>
          {formError && <div className="alert alert-error">{formError}</div>}
          <form onSubmit={(e) => { void handleCreateVersion(e); }}>
            <div className="grid-2">
              <div className="form-group">
                <label className="form-label" htmlFor="ver-version">Version *</label>
                <input
                  id="ver-version"
                  className="form-input"
                  value={form.version}
                  onChange={(e) => setForm((f) => ({ ...f, version: e.target.value }))}
                  placeholder="0.1.0"
                  required
                />
              </div>
              <div className="form-group">
                <label className="form-label" htmlFor="ver-date">Release Date</label>
                <input
                  id="ver-date"
                  type="datetime-local"
                  className="form-input"
                  value={form.releaseDate}
                  onChange={(e) => setForm((f) => ({ ...f, releaseDate: e.target.value }))}
                />
              </div>
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="ver-url">Download URL *</label>
              <input
                id="ver-url"
                type="url"
                className="form-input"
                value={form.downloadUrl}
                onChange={(e) => setForm((f) => ({ ...f, downloadUrl: e.target.value }))}
                placeholder="https://github.com/SemRels/…/releases/download/…"
                required
              />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="ver-changelog">Changelog</label>
              <textarea
                id="ver-changelog"
                className="form-textarea"
                value={form.changelog}
                onChange={(e) => setForm((f) => ({ ...f, changelog: e.target.value }))}
                placeholder="What changed in this version?"
              />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="ver-checksums">
                SHA-256 Checksums (JSON)
              </label>
              <textarea
                id="ver-checksums"
                className="form-textarea"
                value={form.checksums}
                onChange={(e) => setForm((f) => ({ ...f, checksums: e.target.value }))}
                placeholder={'{\n  "linux_amd64": "<sha256>",\n  "darwin_arm64": "<sha256>"\n}'}
                style={{ fontFamily: 'monospace', fontSize: '0.8rem' }}
              />
            </div>

            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={form.prerelease}
                  onChange={(e) => setForm((f) => ({ ...f, prerelease: e.target.checked }))}
                />
                <span className="form-label" style={{ margin: 0 }}>Pre-release</span>
              </label>
            </div>

            <div className="flex-row">
              <button type="submit" className="btn btn-primary" disabled={saving}>
                {saving ? <><span className="spinner" /> Saving…</> : 'Publish Version'}
              </button>
            </div>
          </form>
        </div>
      )}

      {versions.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: '2.5rem', color: 'var(--text-muted)' }}>
          <p>No versions published yet.</p>
          <button
            type="button"
            className="btn btn-primary"
            style={{ marginTop: '0.5rem' }}
            onClick={() => setShowForm(true)}
          >
            + Publish first version
          </button>
        </div>
      ) : (
        <div className="card" style={{ padding: 0 }}>
          <div className="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Version</th>
                  <th>Released</th>
                  <th>Channel</th>
                  <th>Download URL</th>
                  <th>Checksums</th>
                </tr>
              </thead>
              <tbody>
                {versions.map((v) => (
                  <tr key={v.id}>
                    <td><code>v{v.version}</code></td>
                    <td className="text-sm text-muted">
                      {v.releaseDate
                        ? new Intl.DateTimeFormat('en', { dateStyle: 'medium' }).format(new Date(v.releaseDate))
                        : '—'}
                    </td>
                    <td>
                      {v.prerelease
                        ? <span className="badge" style={{ background: 'rgba(251,191,36,0.15)', color: 'var(--warning)' }}>pre-release</span>
                        : <span className="badge" style={{ background: 'rgba(110,231,183,0.15)', color: 'var(--success)' }}>stable</span>}
                    </td>
                    <td className="text-sm" style={{ maxWidth: 280 }}>
                      <a href={v.downloadUrl} target="_blank" rel="noopener" style={{ color: 'var(--accent)', wordBreak: 'break-all' }}>
                        {v.downloadUrl.slice(0, 60)}{v.downloadUrl.length > 60 ? '…' : ''}
                      </a>
                    </td>
                    <td className="text-sm text-muted">
                      {v.checksums ? `${Object.keys(v.checksums).length} platform(s)` : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </>
  );
}
