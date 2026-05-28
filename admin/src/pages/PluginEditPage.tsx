import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getPlugin, createPlugin, updatePlugin } from '../lib/api';
import type { Plugin } from '../lib/api';

const CATS  = ['provider','analyzer','condition','hook','updater'];
const LICS  = ['Apache-2.0','MIT','GPL-3.0','BSD-2-Clause','BSD-3-Clause','MPL-2.0'];

export default function PluginEditPage() {
  const { id } = useParams<{ id: string }>();
  const isNew  = !id;
  const navigate = useNavigate();
  const [form, setForm] = useState<Partial<Plugin>>({ name:'', description:'', author:'', category:'provider', repository:'', license:'Apache-2.0', tags:[] });
  const [tagsInput, setTagsInput] = useState('');
  const [loading, setLoading] = useState(!isNew);
  const [saving, setSaving]   = useState(false);
  const [error, setError]     = useState('');

  useEffect(() => {
    if (!isNew && id) {
      getPlugin(id).then(({ data }) => { setForm(data); setTagsInput((data.tags ?? []).join(', ')); setLoading(false); })
        .catch((e: unknown) => { setError(e instanceof Error ? e.message : 'Failed'); setLoading(false); });
    }
  }, [id, isNew]);

  const set = (f: keyof Plugin, v: string | string[]) => setForm(p => ({ ...p, [f]: v }));

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault(); setSaving(true); setError('');
    const tags = tagsInput.split(',').map(t => t.trim()).filter(Boolean);
    try {
      isNew ? await createPlugin({ ...form, tags }) : await updatePlugin(id!, { ...form, tags });
      navigate('/plugins');
    } catch (e: unknown) { setError(e instanceof Error ? e.message : 'Save failed'); setSaving(false); }
  }

  if (loading) return <div className="page__body muted">Loading…</div>;

  return (
    <>
      <div className="page__header">
        <h1 className="page__title">{isNew ? 'Add Plugin' : `Edit · ${form.name}`}</h1>
        <button type="button" className="btn btn--sm" onClick={() => navigate('/plugins')}>← Back</button>
      </div>
      <div className="page__body">
        {error && <div className="alert alert--error">{error}</div>}
        <div className="card" style={{ maxWidth: 640, margin: '0 auto' }}>
          <form onSubmit={(e) => { void handleSubmit(e); }}>
            <div style={{ display:'grid', gridTemplateColumns:'1fr 1fr', gap:'.75rem' }}>
              <div className="field">
                <label htmlFor="name">Name *</label>
                <input id="name" className="input" value={form.name ?? ''} onChange={e => set('name', e.target.value)} placeholder="provider-github" required disabled={!isNew} />
              </div>
              <div className="field">
                <label htmlFor="category">Category *</label>
                <select id="category" className="select" value={form.category ?? 'provider'} onChange={e => set('category', e.target.value)}>
                  {CATS.map(c => <option key={c} value={c}>{c}</option>)}
                </select>
              </div>
            </div>
            <div className="field">
              <label htmlFor="desc">Description *</label>
              <textarea id="desc" className="textarea" value={form.description ?? ''} onChange={e => set('description', e.target.value)} placeholder="Short description" required />
            </div>
            <div style={{ display:'grid', gridTemplateColumns:'1fr 1fr', gap:'.75rem' }}>
              <div className="field">
                <label htmlFor="author">Author *</label>
                <input id="author" className="input" value={form.author ?? ''} onChange={e => set('author', e.target.value)} placeholder="semrel Authors" required />
              </div>
              <div className="field">
                <label htmlFor="license">License *</label>
                <select id="license" className="select" value={form.license ?? 'Apache-2.0'} onChange={e => set('license', e.target.value)}>
                  {LICS.map(l => <option key={l} value={l}>{l}</option>)}
                </select>
              </div>
            </div>
            <div className="field">
              <label htmlFor="repo">Repository URL</label>
              <input id="repo" type="url" className="input" value={form.repository ?? ''} onChange={e => set('repository', e.target.value)} placeholder="https://github.com/SemRels/…" />
            </div>
            <div className="field">
              <label htmlFor="tags">Tags (comma-separated)</label>
              <input id="tags" className="input" value={tagsInput} onChange={e => setTagsInput(e.target.value)} placeholder="github, provider" />
            </div>
            <div className="flex gap-sm mt-1">
              <button type="submit" className="btn btn--primary" disabled={saving}>{saving ? 'Saving…' : isNew ? 'Create' : 'Save'}</button>
              <button type="button" className="btn" onClick={() => navigate('/plugins')}>Cancel</button>
            </div>
          </form>
        </div>
      </div>
    </>
  );
}
