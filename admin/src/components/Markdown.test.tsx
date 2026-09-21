import { describe, it, expect } from 'vitest';
import { renderMarkdown } from './Markdown';

// Changelogs come from GitHub release bodies, which any plugin author writes.
// marked passes raw HTML through, so without sanitisation these payloads would
// execute in the admin origin.
describe('renderMarkdown', () => {
  it('strips script tags', () => {
    const html = renderMarkdown('# Release\n\n<script>alert(1)</script>');
    expect(html).not.toContain('<script');
    expect(html).toContain('Release');
  });

  it('strips inline event handlers', () => {
    const html = renderMarkdown('<img src=x onerror="alert(1)">');
    expect(html).not.toContain('onerror');
  });

  it('drops javascript: links', () => {
    const html = renderMarkdown('[click me](javascript:alert(1))');
    expect(html).not.toContain('javascript:');
  });

  it('drops remote images used as tracking pixels', () => {
    const html = renderMarkdown('![pixel](https://tracker.example/p.gif)');
    expect(html).not.toContain('<img');
  });

  it('strips iframes', () => {
    const html = renderMarkdown('<iframe src="https://evil.example"></iframe>');
    expect(html).not.toContain('<iframe');
  });

  it('keeps ordinary formatting intact', () => {
    const html = renderMarkdown('## Fixed\n\n- **bold** and `code`\n- [link](https://example.com)');
    expect(html).toContain('<h2');
    expect(html).toContain('<strong>bold</strong>');
    expect(html).toContain('<code>code</code>');
    expect(html).toContain('href="https://example.com"');
  });

  it('returns an empty string for empty input', () => {
    expect(renderMarkdown('')).toBe('');
  });
});
