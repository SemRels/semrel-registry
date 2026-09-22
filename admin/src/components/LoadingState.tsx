/**
 * Shared loading and empty presentations.
 *
 * These replace bare "Loading…" paragraphs. A skeleton that matches the shape
 * of what is coming tells the reader what to expect and stops the page
 * jumping when it arrives; a paragraph tells them only that something is
 * happening somewhere.
 */

interface SkeletonProps {
  /** How many placeholder rows or cards to draw. */
  readonly count?: number;
  /** Accessible description of what is loading. */
  readonly label: string;
}

/** Placeholder rows shaped like a table. */
export function TableSkeleton({ count = 5, label }: SkeletonProps) {
  return (
    <div className="skeleton-stack" aria-busy="true">
      {/* The live region carries the announcement; the shapes are decorative. */}
      <span className="sr-only" role="status">{label}</span>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="skeleton skeleton-row" aria-hidden="true" />
      ))}
    </div>
  );
}

/** Placeholder blocks shaped like stat tiles. */
export function TileSkeleton({ count = 4, label }: SkeletonProps) {
  return (
    <div className="skeleton-tiles" aria-busy="true">
      <span className="sr-only" role="status">{label}</span>
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="skeleton skeleton-tile" aria-hidden="true" />
      ))}
    </div>
  );
}

interface EmptyStateProps {
  readonly title: string;
  readonly children?: React.ReactNode;
  /** A single next step, when there is an obvious one. */
  readonly action?: React.ReactNode;
}

/**
 * An empty list with an explanation and, where one exists, the next step.
 *
 * "No versions yet." leaves the reader to work out whether that is a problem
 * and what to do about it.
 */
export function EmptyState({ title, children, action }: EmptyStateProps) {
  return (
    <div className="empty-state">
      <span className="empty-state__title">{title}</span>
      {children && <p style={{ margin: 0, maxWidth: '34rem' }}>{children}</p>}
      {action}
    </div>
  );
}
