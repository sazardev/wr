import { describe, expect, it } from 'vitest';
import { searchEntries, snippetFor, tokenize } from './search';

const entries = [
  { doc: 'guide', section: 'start', title: 'Start', url: '/docs/guide/start/', text: 'Install wr and read your first page.' },
  {
    doc: 'guide',
    section: 'following-links',
    title: 'Following links',
    url: '/docs/guide/following-links/',
    text: 'Press f to label every link on the screen. Type the label to open that link. Tab focuses links one by one.',
  },
  {
    doc: 'reference',
    section: 'shortcuts',
    title: 'Shortcuts',
    url: '/docs/reference/shortcuts/',
    text: 'f: label the links on screen. t: section index. H and L: back and forward.',
  },
] as const;

describe('tokenize', () => {
  it('splits on anything that is not a letter or number', () => {
    expect(tokenize('press f to label! links')).toEqual(['press', 'to', 'label', 'links']);
  });

  it('ignores one-character words', () => {
    expect(tokenize('a b cd')).toEqual(['cd']);
  });
});

describe('searchEntries', () => {
  it('returns nothing for an empty query', () => {
    expect(searchEntries(entries, '  ')).toEqual([]);
  });

  it('ranks a title match above body matches', () => {
    const hits = searchEntries(entries, 'links');
    expect(hits[0]!.entry.title).toBe('Following links');
    expect(hits.map((h) => h.entry.section)).toContain('shortcuts');
  });

  it('requires every word to be present', () => {
    expect(searchEntries(entries, 'links banana')).toEqual([]);
  });

  it('scopes the search to one document', () => {
    const hits = searchEntries(entries, 'links', { scope: 'reference' });
    expect(hits).toHaveLength(1);
    expect(hits[0]!.entry.doc).toBe('reference');
  });

  it('shows a window around the match', () => {
    const hits = searchEntries(entries, 'bookmark?? no: tab');
    expect(hits).toEqual([]);
    const hit = searchEntries(entries, 'tab')[0]!;
    expect(hit.snippet.toLowerCase()).toContain('tab');
  });
});

describe('snippetFor', () => {
  it('keeps the beginning when the match is early', () => {
    expect(snippetFor('Install wr and read', 'install')).toBe('Install wr and read');
  });

  it('centers on the match and cuts with an ellipsis', () => {
    const long = `${'x'.repeat(200)} target ${'y'.repeat(200)}`;
    const snippet = snippetFor(long, 'target');
    expect(snippet.startsWith('…')).toBe(true);
    expect(snippet.endsWith('…')).toBe(true);
    expect(snippet).toContain('target');
  });
});
