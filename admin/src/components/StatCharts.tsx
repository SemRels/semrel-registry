import { useId, useState } from 'react';

/**
 * Donut charts for the dashboard's structured numbers.
 *
 * The four headline totals stay stat tiles: a single current value has no
 * shape. What gets a chart is the data that splits into parts — plugins per
 * category, and the review status of the catalogue.
 */

export interface Slice {
  readonly key: string;
  readonly label: string;
  readonly value: number;
}

interface DonutProps {
  readonly title: string;
  readonly slices: Slice[];
  /** Fill per slice key. */
  readonly colorOf: (slice: Slice, index: number) => string;
  /** What the centre counts, e.g. "plugins". */
  readonly unit: string;
}

const SIZE = 168;
const STROKE = 26;
const RADIUS = (SIZE - STROKE) / 2;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;
/** Surface gap between neighbouring segments, in path units. */
const GAP = 3;

/**
 * A donut with a legend that names, counts and shares out every slice.
 *
 * The legend is not decoration: several palette steps sit below 3:1 against a
 * light surface, so identity has to be carried by text as well as by fill. It
 * doubles as the table view for anyone who cannot read the arcs.
 */
function Donut({ title, slices, colorOf, unit }: DonutProps) {
  const [hovered, setHovered] = useState<string | null>(null);
  const headingId = useId();

  const positive = slices.filter(slice => slice.value > 0);
  const total = positive.reduce((sum, slice) => sum + slice.value, 0);
  if (total === 0) return null;

  // Walk the circle once, converting each value into an arc length and the
  // offset it starts at.
  let cursor = 0;
  const arcs = positive.map((slice, index) => {
    const fraction = slice.value / total;
    const length = fraction * CIRCUMFERENCE;
    // A single slice would otherwise be a full ring with a notch cut out of it.
    const drawn = positive.length === 1 ? length : Math.max(1, length - GAP);
    const arc = {
      slice,
      color: colorOf(slice, index),
      dash: `${drawn} ${CIRCUMFERENCE - drawn}`,
      // Negative offset winds clockwise from twelve o'clock.
      offset: -cursor,
      share: Math.round(fraction * 100),
    };
    cursor += length;
    return arc;
  });

  const active = arcs.find(arc => arc.slice.key === hovered);
  const centreValue = active ? active.slice.value : total;
  const centreLabel = active ? active.slice.label : `total ${unit}`;

  return (
    <figure className="viz" aria-labelledby={headingId}>
      <figcaption id={headingId} className="viz__title">{title}</figcaption>

      <div className="viz-donut">
        <svg
          viewBox={`0 0 ${SIZE} ${SIZE}`}
          width={SIZE}
          height={SIZE}
          className="viz-donut__svg"
          role="img"
          aria-label={`${title}: ${arcs.map(a => `${a.slice.label} ${a.slice.value}`).join(', ')}`}
        >
          {/* Track, so a mostly-empty donut still reads as a ring. */}
          <circle
            cx={SIZE / 2}
            cy={SIZE / 2}
            r={RADIUS}
            fill="none"
            stroke="var(--surface-subtle)"
            strokeWidth={STROKE}
          />
          <g transform={`rotate(-90 ${SIZE / 2} ${SIZE / 2})`}>
            {arcs.map(arc => (
              <circle
                key={arc.slice.key}
                cx={SIZE / 2}
                cy={SIZE / 2}
                r={RADIUS}
                fill="none"
                stroke={arc.color}
                strokeWidth={hovered === arc.slice.key ? STROKE + 5 : STROKE}
                strokeDasharray={arc.dash}
                strokeDashoffset={arc.offset}
                className="viz-donut__arc"
                onMouseEnter={() => setHovered(arc.slice.key)}
                onMouseLeave={() => setHovered(null)}
              />
            ))}
          </g>

          {/* The hole is the donut's one advantage over a pie: it carries the
              total, and the hovered slice while one is hovered. */}
          <text
            x={SIZE / 2}
            y={SIZE / 2 - 2}
            textAnchor="middle"
            className="viz-donut__value"
          >
            {centreValue.toLocaleString()}
          </text>
          <text
            x={SIZE / 2}
            y={SIZE / 2 + 15}
            textAnchor="middle"
            className="viz-donut__caption"
          >
            {centreLabel}
          </text>
        </svg>

        <ul className="viz-legend viz-legend--column">
          {arcs.map(arc => (
            <li
              key={arc.slice.key}
              className={hovered === arc.slice.key ? 'viz-legend__item is-hovered' : 'viz-legend__item'}
              onMouseEnter={() => setHovered(arc.slice.key)}
              onMouseLeave={() => setHovered(null)}
              onFocus={() => setHovered(arc.slice.key)}
              onBlur={() => setHovered(null)}
              tabIndex={0}
              aria-label={`${arc.slice.label}: ${arc.slice.value} ${unit}, ${arc.share} percent`}
            >
              <span className="viz-legend__swatch" style={{ background: arc.color }} aria-hidden="true" />
              <span className="viz-legend__label">{arc.slice.label}</span>
              <span className="viz-legend__value">{arc.slice.value.toLocaleString()} · {arc.share}%</span>
            </li>
          ))}
        </ul>
      </div>
    </figure>
  );
}

/**
 * The categorical order is fixed and never cycled: slot 1 is always the first
 * category, so a filter that removes one does not repaint the rest. A ninth
 * category would fold into the tail rather than inventing a hue — under CVD a
 * generated ninth is indistinguishable from one already on screen.
 */
const CATEGORY_SLOTS = [
  'var(--viz-cat-1)', 'var(--viz-cat-2)', 'var(--viz-cat-3)', 'var(--viz-cat-4)',
  'var(--viz-cat-5)', 'var(--viz-cat-6)', 'var(--viz-cat-7)', 'var(--viz-cat-8)',
];

export function CategoryDonut({ data }: Readonly<{ data: Slice[] }>) {
  return (
    <Donut
      title="Plugins per category"
      slices={data}
      unit="plugins"
      colorOf={(_, index) => CATEGORY_SLOTS[index % CATEGORY_SLOTS.length]}
    />
  );
}

/** Status colours are reserved and never themed: they mean the same everywhere. */
const STATUS_FILL: Record<string, string> = {
  active: 'var(--status-good)',
  pending: 'var(--status-warning)',
  rejected: 'var(--status-critical)',
};

const STATUS_LABEL: Record<string, string> = {
  active: 'Active',
  pending: 'Pending',
  rejected: 'Rejected',
};

export function StatusDonut({ counts }: Readonly<{ counts: Record<string, number> }>) {
  const slices: Slice[] = Object.entries(counts).map(([key, value]) => ({
    key,
    label: STATUS_LABEL[key] ?? key,
    value: Number(value ?? 0),
  }));

  return (
    <Donut
      title="Review status"
      slices={slices}
      unit="plugins"
      colorOf={slice => STATUS_FILL[slice.key] ?? 'var(--text-muted)'}
    />
  );
}
