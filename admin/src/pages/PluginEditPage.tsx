import { useEffect, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { getPlugin, createPlugin, updatePlugin } from '../lib/api';
import type { Plugin } from '../lib/api';

const CATEGORIES = ['provider', 'analyzer', 'condition', 'hook', 'updater'];
const LICENSES = ['Apache-2.0', 'MIT', 'GPL-3.0', 'BSD-2-Clause', 'BSD-3-Clause', 'MPL-2.0', 'LGPL-3.0'];

export default function PluginEditPage() {
  const { id } = useParams<{ id: string }>();
  const isNew = !id;
  const navigate = useNavigate();

  const [form, setForm] = useState<Partial<Plugin>>({
    name: '',
    description: '',
    author: '',
    category: 'provider',
    repository: '',
    license: 'Apache-2.0',
    tags: [],
  });
  const [tagsInput, setTagsInput] = useState('');
  const [loading, setLoading] = useState(!isNew);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!isNew && id) {
      getPlugin(id)
        .then(({ data }) => {
          setForm(data);
          setTagsInput((data.tags ?? []).join(', '));
          setLoading(false);
        })
        .catch((e: unknown) => {
          setError(e instanceof Error ? e.message : 'Failed to load plugin');
          setLoading(false);
        });
    }
  }, [id, isNew]);

  function handleChange(field: keyof Plugin, value: string | string[]) {
    setForm((prev) => ({ ...prev, [field]: value }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError('');

    const tags = tagsInput
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);

    const payload = { ...form, tags };

    try {
      if (isNew) {
        await createPlugin(payload);
      } else {
        await updatePlugin(id!, payload);
      }
      navigate('/plugins');
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Save failed');
      setSaving(false);
    }
  }

  if (loading) {
    return <div className="loading-block"><span className="spinner" /> Loading plugin…</div>;
  }

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

      <h1 className="page-title">{isNew ? 'Add Plugin' : `Edit · ${form.name}`}</h1>
      <p className="page-subtitle">{isNew ? 'Register a new plugin in the registry.' : 'Update plugin metadata.'}</p>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="card" style={{ maxWidth: 680 }}>
        <form onSubmit={(e) => { void handleSubmit(e); }}>
          <div className="grid-2">
            <div className="form-group">
              <label className="form-label" htmlFor="name">Name *</label>
              <input
                id="name"
                className="form-input"
                value={form.name ?? ''}
                onChange={(e) => handleChange('name', e.target.value)}
                placeholder="provider-github"
                required
                disabled={!isNew}
              />
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="category">Category *</label>
              <select
                id="category"
                className="form-select"
                value={form.category ?? 'provider'}
                onChange={(e) => handleChange('category', e.target.value)}
                required
              >
                {CATEGORIES.map((c) => (
                  <option key={c} value={c}>{c}</option>
                ))}
              </select>
            </div>
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="description">Description *</label>
            <textarea
              id="description"
              className="form-textarea"
              value={form.description ?? ''}
              onChange={(e) => handleChange('description', e.target.value)}
              placeholder="Short description of what this plugin does."
              required
            />
          </div>

          <div className="grid-2">
            <div className="form-group">
              <label className="form-label" htmlFor="author">Author *</label>
              <input
                id="author"
                className="form-input"
                value={form.author ?? ''}
                onChange={(e) => handleChange('author', e.target.value)}
                placeholder="semrel Authors"
                required
              />
            </div>
            <div className="form-group">
              <label className="form-label" htmlFor="license">License *</label>
              <select
                id="license"
                className="form-select"
                value={form.license ?? 'Apache-2.0'}
                onChange={(e) => handleChange('license', e.target.value)}
                required
              >
                {LICENSES.map((l) => (
                  <option key={l} value={l}>{l}</option>
                ))}
              </select>
            </div>
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="repository">Repository URL</label>
            <input
              id="repository"
              type="url"
              className="form-input"
              value={form.repository ?? ''}
              onChange={(e) => handleChange('repository', e.target.value)}
              placeholder="https://github.com/SemRels/provider-github"
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="tags">Tags (comma-separated)</label>
            <input
              id="tags"
              className="form-input"
              value={tagsInput}
              onChange={(e) => setTagsInput(e.target.value)}
              placeholder="github, provider, releases"
            />
          </div>

          <div className="flex-row" style={{ marginTop: '0.5rem' }}>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? <><span className="spinner" /> Saving…</> : (isNew ? 'Create Plugin' : 'Save Changes')}
            </button>
            <button
              type="button"
              className="btn btn-ghost"
              onClick={() => navigate('/plugins')}
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </>
  );
}
