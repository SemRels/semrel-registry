import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getPlugin, listVersions, createVersion, deleteVersion, yankVersion, unyankVersion } from '../lib/api';
import type { Plugin, PluginVersion } from '../lib/api';
import Markdown from '../components/Markdown';
import DeletionConfirmDialog from '../components/DeletionConfirmDialog';
import ReasonDialog from '../components/ReasonDialog';
import { TableSkeleton, EmptyState } from '../components/LoadingState';

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
  const [deleteTarget, setDeleteTarget] = useState<PluginVersion | null>(null);
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [deleteError, setDeleteError] = useState('');
  const [yankTarget, setYankTarget] = useState<PluginVersion | null>(null);
  const [yankBusy, setYankBusy] = useState(false);
  const [yankError, setYankError] = useState('');
  const [notice, setNotice] = useState('');

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

  async function handleYank(reason: string) {
    if (!yankTarget) return;
    setYankBusy(true);
    setYankError('');
    try {
      const updated = await yankVersion(id!, yankTarget.id, reason);
      setVersions(prev => prev.map(v => v.id === updated.id ? updated : v));
      setNotice(`v${yankTarget.version} is yanked. Existing pinned installs still resolve it.`);
      setYankTarget(null);
    } catch (e: unknown) {
      setYankError(e instanceof Error ? e.message : 'Yank failed');
    } finally {
      setYankBusy(false);
    }
  }

  async function handleUnyank(version: PluginVersion) {
    setYankBusy(true);
    setError('');
    try {
      const updated = await unyankVersion(id!, version.id);
      setVersions(prev => prev.map(v => v.id === updated.id ? updated : v));
      setNotice(`v${version.version} is installable again.`);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Un-yank failed');
    } finally {
      setYankBusy(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleteBusy(true);
    setDeleteError('');
    try {
      await deleteVersion(id!, deleteTarget.id, {
        confirmation: `${plugin?.namespace ? `${plugin.namespace}/` : ''}${plugin?.name ?? id}@${deleteTarget.version}`,
      });
      setVersions(prev => prev.filter(x => x.id !== deleteTarget.id));
      setDeleteTarget(null);
    } catch (e: unknown) {
      setDeleteError(e instanceof Error ? e.message : 'Delete failed');
    } finally {
      setDeleteBusy(false);
    }
  }

  if (loading) {
    return (
      <div className="page__body">
        <TableSkeleton label="Loading versions" count={4} />
      </div>
    );
  }

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">Versions · <span className="text-accent">{plugin?.name}</span></h1>
        <div className="flex gap-sm">
          <button type="button" className="btn btn--sm" onClick={() => navigate('/admin/plugins')}>← Back</button>
          <button type="button" className="btn btn--primary btn--sm" onClick={() => setShowForm(s => !s)}>
            {showForm ? 'Cancel' : '+ Add Version'}
          </button>
        </div>
      </div>
      <div className="page__body">
        {error && <div className="alert alert--error" role="alert">{error}</div>}
        {notice && <div className="alert alert--info" role="status">{notice}</div>}
        <p className="muted text-xs mb-2">
          <strong>Yank</strong> retracts a release without breaking builds that pin it — it stops
          being offered for new installs and the reason is shown to anyone using it.
          <strong> Delete</strong> removes it outright and does break those builds; it requires
          typing the release tag to confirm.
        </p>

        {showForm && (
          <div className="card form-card">
            <h2 className="mb-md">New Version</h2>
            {formError && <div className="alert alert--error">{formError}</div>}
            <form onSubmit={(e) => { void handleCreate(e); }}>
              <div className="form-grid-2">
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
                <textarea id="cs" className="textarea textarea--mono" value={form.checksums} onChange={e => setForm(f=>({...f,checksums:e.target.value}))} placeholder={'{"linux_amd64":"<sha256>"}'} /></div>
              <label className="checkbox-label">
                <input type="checkbox" checked={form.prerelease} onChange={e => setForm(f=>({...f,prerelease:e.target.checked}))} />
                {' '}Pre-release
              </label>
              <button type="submit" className="btn btn--primary" disabled={saving}>{saving ? 'Saving…' : 'Publish'}</button>
            </form>
          </div>
        )}

        {versions.length === 0 ? (
          <EmptyState
            title="No versions published yet"
            action={<button type="button" className="btn btn--primary" onClick={() => setShowForm(true)}>Publish the first version</button>}
          >
            A plugin without a published version cannot be installed. Versions
            usually arrive automatically from GitHub releases; you can also add
            one by hand.
          </EmptyState>
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
                          className="version-toggle"
                          title={v.changelog ? 'Click to view release notes' : 'No release notes'}
                        >
                          {expandedId === v.id ? '▾' : '▸'} v{v.version}
                        </button>
                      </td>
                      <td data-label="Released" className="muted">{v.releaseDate ? new Intl.DateTimeFormat('en',{dateStyle:'medium'}).format(new Date(v.releaseDate)) : '—'}</td>
                      <td data-label="Channel">
                        {v.yankedAt
                          ? <span className="badge badge--yanked" title={v.yankedReason}>yanked</span>
                          : v.prerelease
                            ? <span className="badge badge--pre">pre</span>
                            : <span className="badge badge--stable">stable</span>
                        }
                      </td>
                      <td data-label="Views">{Number(v.views ?? 0).toLocaleString()}</td>
                      <td data-label="Downloads">{Number(v.downloads ?? 0).toLocaleString()}</td>
                      <td data-label="Download" className="muted truncate text-xs max-w-200">
                        <a
                          href={`/api/v1/plugins/${encodeURIComponent(id ?? '')}/versions/${encodeURIComponent(v.version)}/download`}
                          target="_blank"
                          rel="noopener"
                        >
                          Download via registry
                        </a>
                      </td>
                      <td data-label="Platforms" className="muted">{v.checksums ? Object.keys(v.checksums).length : 0}</td>
                      <td data-label="Actions">
                        <div className="flex gap-xs flex-wrap">
                          {/* Yank is the safe retraction: consumers who already
                              pin this version keep resolving it, they just stop
                              being offered it. Delete breaks those builds, so it
                              is the secondary action, not the primary one. */}
                          {v.yankedAt ? (
                            <button
                              type="button"
                              className="btn btn--sm"
                              onClick={() => { void handleUnyank(v); }}
                              disabled={yankBusy}
                              title="Make this version installable again"
                            >
                              Un-yank
                            </button>
                          ) : (
                            <button
                              type="button"
                              className="btn btn--sm"
                              onClick={() => { setYankError(''); setYankTarget(v); }}
                              title="Retract this version without breaking pinned installs"
                            >
                              Yank
                            </button>
                          )}
                          <button
                            type="button"
                            className="btn btn--sm btn--danger"
                            onClick={() => { setDeleteError(''); setDeleteTarget(v); }}
                            title="Permanently remove this version — breaks builds that pin it"
                          >
                            Delete
                          </button>
                        </div>
                      </td>
                    </tr>
                    {expandedId === v.id && (
                      <tr key={`${v.id}-notes`}>
                        <td colSpan={8} className="version-notes-cell">
                          {v.changelog ? (
                            <Markdown
                              source={v.changelog}
                              className="prose prose--version-notes"
                            />
                          ) : (
                            <span className="muted">No release notes for this version.</span>
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
      <ReasonDialog
        open={yankTarget !== null}
        title={yankTarget ? `Yank v${yankTarget.version}?` : 'Yank version?'}
        message="The version stays resolvable, so builds that already pin it keep working. It simply stops being offered for new installs, and the reason is shown to anyone using it."
        label="Why is this version being yanked?"
        hint="Shown to everyone who has this version pinned."
        placeholder="e.g. The linux-amd64 binary was built from the wrong commit."
        confirmLabel="Yank version"
        busyLabel="Yanking…"
        busy={yankBusy}
        error={yankError}
        onClose={() => { if (!yankBusy) { setYankError(''); setYankTarget(null); } }}
        onConfirm={(reason) => { void handleYank(reason); }}
      />

      <DeletionConfirmDialog
        open={deleteTarget !== null}
        title={deleteTarget ? `Delete v${deleteTarget.version}?` : 'Delete version?'}
        message={deleteTarget && plugin
          ? `This removes version v${deleteTarget.version} of ${plugin.name} from the registry. Anyone who pins this version will no longer be able to install it. If you only want to stop recommending it, yank it instead.`
          : ''}
        confirmationValue={deleteTarget && plugin
          ? `${plugin.namespace ? `${plugin.namespace}/` : ''}${plugin.name}@${deleteTarget.version}`
          : ''}
        confirmationLabel="Plugin version reference"
        confirmLabel="Delete version"
        busyLabel="Deleting…"
        acknowledgement="I understand this breaks installs that pin this version."
        busy={deleteBusy}
        error={deleteError}
        onClose={() => { if (!deleteBusy) { setDeleteError(''); setDeleteTarget(null); } }}
        onConfirm={() => { void handleDelete(); }}
      />
    </>
  );
}
