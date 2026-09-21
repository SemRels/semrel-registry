import { useEffect, useState } from 'react';
import { getPluginReadme } from '../lib/api';
import type { PluginReadme } from '../lib/api';
import Markdown from '../components/Markdown';

/**
 * Renders the plugin repository's README.
 *
 * The content is author-supplied, so it goes through the sanitising Markdown
 * component. A missing README is an ordinary state — plenty of plugins have
 * none — so the section simply does not appear rather than showing an error.
 */
export default function ReadmeSection({
  pluginId,
  repository,
}: {
  readonly pluginId: number | string;
  readonly repository?: string;
}) {
  const [readme, setReadme] = useState<PluginReadme | null>(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    getPluginReadme(pluginId)
      .then(result => { if (!cancelled) setReadme(result); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [pluginId]);

  if (loading) {
    return (
      <section className="card" style={{ padding: '1.25rem', marginBottom: '1.25rem' }} aria-busy="true">
        <h2 style={{ margin: '0 0 .75rem', fontSize: 'var(--fs-md)', fontWeight: 700 }}>Documentation</h2>
        <div className="skeleton" style={{ height: '6rem' }} />
      </section>
    );
  }

  if (!readme?.markdown) return null;

  return (
    <section className="card" style={{ padding: '1.25rem', marginBottom: '1.25rem' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '.5rem', flexWrap: 'wrap', marginBottom: '.75rem' }}>
        <h2 style={{ margin: 0, fontSize: 'var(--fs-md)', fontWeight: 700 }}>Documentation</h2>
        <a
          href={readme.source || repository}
          target="_blank"
          rel="noopener noreferrer"
          style={{ fontSize: 'var(--fs-xs)' }}
        >
          Read on GitHub <span aria-hidden="true">↗</span>
          <span className="sr-only">(opens in a new tab)</span>
        </a>
      </div>

      {/* A long README would push the version table off the page, so it is
          clipped with an explicit control rather than scrolled inside a box —
          nested scroll regions are hard to reach by keyboard and on touch. */}
      <div className={expanded ? 'readme' : 'readme readme--clipped'}>
        <Markdown source={readme.markdown} />
      </div>

      <button
        type="button"
        className="btn btn--secondary btn--sm"
        style={{ marginTop: '.75rem' }}
        aria-expanded={expanded}
        onClick={() => setExpanded(open => !open)}
      >
        {expanded ? 'Show less' : 'Show full README'}
      </button>
    </section>
  );
}
