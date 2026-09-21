/**
 * A pass/fail marker with a text equivalent.
 *
 * The check and cross glyphs carry their meaning through shape and colour
 * alone, which fails WCAG 1.4.1 for colour-blind users and tells a screen
 * reader nothing useful — some readers announce "✓" as nothing at all. The
 * visible glyph is therefore hidden from assistive technology and paired with
 * a screen-reader-only word.
 */
export default function StatusIcon({
  passed,
  passedLabel = 'Passed',
  failedLabel = 'Failed',
}: {
  readonly passed: boolean;
  readonly passedLabel?: string;
  readonly failedLabel?: string;
}) {
  return (
    <>
      <span aria-hidden="true" style={{ color: passed ? 'var(--success)' : 'var(--danger)', fontWeight: 700, flexShrink: 0 }}>
        {passed ? '✓' : '✗'}
      </span>
      <span className="sr-only">{passed ? passedLabel : failedLabel}: </span>
    </>
  );
}
