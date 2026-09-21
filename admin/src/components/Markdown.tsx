import { useMemo } from 'react';
import { marked } from 'marked';
import DOMPurify, { type Config } from 'dompurify';

/**
 * Sanitisation profile for registry-supplied markdown.
 *
 * Changelogs and descriptions come from GitHub release bodies, which any plugin
 * author controls. `marked` passes raw HTML through by design — it dropped its
 * own sanitiser in v5 — so rendering its output with dangerouslySetInnerHTML
 * hands that author script execution in the admin origin. DOMPurify is what
 * makes the content safe to inject.
 *
 * The allowlist is deliberately narrow: formatting, links, lists, tables and
 * code. No <img> (a remote image is a visitor-tracking pixel), no <iframe>, no
 * <style>, no event handlers, and no javascript:/data: URLs.
 */
const SANITIZE_CONFIG: Config = {
  ALLOWED_TAGS: [
    'p', 'br', 'hr', 'strong', 'em', 'del', 's', 'blockquote',
    'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
    'ul', 'ol', 'li', 'code', 'pre', 'a',
    'table', 'thead', 'tbody', 'tr', 'th', 'td',
  ],
  ALLOWED_ATTR: ['href', 'title', 'align'],
  ALLOW_DATA_ATTR: false,
  // Only these schemes may appear in an href.
  ALLOWED_URI_REGEXP: /^(?:https?|mailto):/i,
};

export function renderMarkdown(source: string): string {
  if (!source) return '';
  const html = marked.parse(source, { async: false }) as string;
  // String(): the declared return type widens to TrustedHTML when the browser
  // supports Trusted Types; the value is a string in either case.
  return String(DOMPurify.sanitize(html, SANITIZE_CONFIG));
}

interface MarkdownProps {
  readonly source: string;
  readonly className?: string;
  readonly style?: React.CSSProperties;
}

/** Renders untrusted markdown. Links open in a new tab. */
export default function Markdown({ source, className, style }: MarkdownProps) {
  const html = useMemo(() => renderMarkdown(source), [source]);

  return (
    <div
      className={className ? `markdown-body ${className}` : 'markdown-body'}
      style={style}
      // Safe: renderMarkdown sanitises with the allowlist above.
      dangerouslySetInnerHTML={{ __html: html }}
      onClick={e => {
        const anchor = (e.target as HTMLElement).closest('a');
        if (anchor) {
          anchor.target = '_blank';
          anchor.rel = 'noopener noreferrer';
        }
      }}
    />
  );
}
