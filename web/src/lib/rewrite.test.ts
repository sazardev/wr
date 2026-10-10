import { describe, expect, it } from 'vitest';
import { rewriteRepoLinks, rewriteSectionAnchors } from './rewrite';

const slugs = new Set(['start', 'the-screen']);

describe('rewriteSectionAnchors', () => {
  const html = '<a href="#start">a</a><a href="#a-heading-inside">b</a>';

  it('turns anchors that name another section into its route', () => {
    expect(rewriteSectionAnchors(html, 'guide', slugs)).toContain('href="/docs/guide/start/"');
  });

  it('leaves anchors that stay inside the page alone', () => {
    expect(rewriteSectionAnchors(html, 'guide', slugs)).toContain('href="#a-heading-inside"');
  });
});

describe('rewriteRepoLinks', () => {
  it('points relative Markdown links at GitHub', () => {
    expect(rewriteRepoLinks('<a href="../README.md#install">install</a>')).toBe(
      '<a href="https://github.com/sazardev/wr/blob/main/README.md#install">install</a>',
    );
  });

  it('leaves external and absolute links alone', () => {
    expect(rewriteRepoLinks('<a href="https://x.dev/a.md">x</a>')).toBe('<a href="https://x.dev/a.md">x</a>');
  });
});
