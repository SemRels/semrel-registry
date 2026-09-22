import { useId, useState } from 'react';

/**
 * Charts for the dashboard's structured numbers.
 *
 * The four headline totals stay stat tiles on purpose: a single current value
 * has no shape, and a one-bar bar chart says less than the number does. What
 * gets a chart here is the data that carries structure — a magnitude comparison
 * across categories, and a part-to-whole split of review status.
 */

interface CategoryDatum {
  readonly label: string;
  readonly value: number;
}

/**
 * Magnitude across plugin categories.
 *
 * A bar chart, not a donut: there are eight categories with word-length names,
 * and eight wedges means eight hues to tell apart plus labels that do not fit.
 * A bar puts them on a shared baseline where the comparison is the length, and
 * the whole set needs one colour rather than eight.
 */
export function CategoryBars({ data, total }: Readonly<{ data: CategoryDatum[]; total: number }>) {
  const [hovered, setHovered] = useState<string | null>(null);
  const headingId = useId();

  if (data.length === 0) return null;

  const sorted = [...data].sort((a, b) => b.value - a.value);
  const max = Math.max(1, ...sorted.map(d => d.value));

  return (
    <figure className="viz" aria-labelledby={headingId}>
      <figcaption id={headingId} className="viz__title">Plugins per category</figcaption>

      <ul className="viz-bars">
        {sorted.map(({ label, value }) => {
          const share = total > 0 ? Math.round((value / total) * 100) : 0;
          return (
            <li
              key={label}
              className="viz-bars__row"
              onMouseEnter={() => setHovered(label)}
              onMouseLeave={() => setHovered(null)}
              onFocus={() => setHovered(label)}
              onBlur={() => setHovered(null)}
              tabIndex={0}
              // The row is the hit target, which is taller than the bar itself.
              aria-label={`${label}: ${value} plugins, ${share} percent of the catalogue`}
            >
              <span className="viz-bars__label">{label}</span>
              <span className="viz-bars__track">
                <span
                  className="viz-bars__fill"
                  style={{ width: `${Math.max(1, (value / max) * 100)}%` }}
                />
              </span>
              {/* Direct label in text ink, not the series colour. */}
              <span className="viz-bars__value">{value.toLocaleString()}</span>

              {hovered === label && (
                <span className="viz-tooltip" role="presentation">
                  {value.toLocaleString()} of {total.toLocaleString()} · {share}%
                </span>
              )}
            </li>
          );
        })}
      </ul>
    </figure>
  );
}

/**
 * Status colours are reserved and never themed: good, warning and critical mean
 * the same thing on every surface. The warning step sits below 3:1 against a
 * light surface, which is a documented property of that palette — so every
 * segment carries a visible label and count, and identity is never colour alone.
 */
const STATUS_STYLE: Record<string, { fill: string; label: string; description: string }> = {
  active:   { fill: 'var(--status-good)',     label: 'Active',   description: 'published in the catalogue' },
  pending:  { fill: 'var(--status-warning)',  label: 'Pending',  description: 'waiting for review' },
  rejected: { fill: 'var(--status-critical)', label: 'Rejected', description: 'turned down, with a reason' },
};

/**
 * Review status as a part-to-whole split.
 *
 * A single stacked bar rather than a donut: the three parts sum to the whole
 * catalogue, and a bar compares segment lengths on one axis instead of asking
 * the reader to judge angles.
 */
export function StatusComposition({ counts }: Readonly<{ counts: Record<string, number> }>) {
  const [hovered, setHovered] = useState<string | null>(null);
  const headingId = useId();

  const entries = Object.entries(counts)
    .map(([status, value]) => ({ status, value: Number(value ?? 0) }))
    .filter(entry => entry.value > 0);

  const total = entries.reduce((sum, entry) => sum + entry.value, 0);
  if (total === 0) return null;

  return (
    <figure className="viz" aria-labelledby={headingId}>
      <figcaption id={headingId} className="viz__title">Review status</figcaption>

      <div className="viz-stack" role="img" aria-label={
        entries.map(e => `${STATUS_STYLE[e.status]?.label ?? e.status}: ${e.value}`).join(', ')
      }>
        {entries.map(({ status, value }) => (
          <span
            key={status}
            className="viz-stack__segment"
            style={{
              flexGrow: value,
              background: STATUS_STYLE[status]?.fill ?? 'var(--text-muted)',
            }}
            onMouseEnter={() => setHovered(status)}
            onMouseLeave={() => setHovered(null)}
          />
        ))}
      </div>

      {/* The legend is the relief the contrast warning requires: every segment
          is named and counted in text, so nothing depends on the fill alone. */}
      <ul className="viz-legend">
        {entries.map(({ status, value }) => {
          const style = STATUS_STYLE[status];
          const share = Math.round((value / total) * 100);
          return (
            <li
              key={status}
              className={hovered === status ? 'viz-legend__item is-hovered' : 'viz-legend__item'}
              onMouseEnter={() => setHovered(status)}
              onMouseLeave={() => setHovered(null)}
            >
              <span
                className="viz-legend__swatch"
                style={{ background: style?.fill ?? 'var(--text-muted)' }}
                aria-hidden="true"
              />
              <span className="viz-legend__label">{style?.label ?? status}</span>
              <span className="viz-legend__value">{value.toLocaleString()} · {share}%</span>
              <span className="sr-only">{style?.description}</span>
            </li>
          );
        })}
      </ul>
    </figure>
  );
}
