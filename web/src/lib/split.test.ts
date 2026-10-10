import { describe, expect, it } from 'vitest';
import { leadParagraph, splitDocument, plainText } from './split';

const guide = `# wr guide

How to read and browse the web from your terminal with \`wr\`.

- [Start](#start)
- [The screen](#the-screen)
- [Reading a page](#reading-a-page)

## Start

Install it:

\`\`\`sh
curl -fsSL https://example.com | sh
\`\`\`

## The screen

One paragraph.

\`\`\`text
  ## this fence is not a heading
\`\`\`

## Local files, Markdown and other content

Last one.
`;

describe('plainText', () => {
  it('removes inline markdown syntax', () => {
    expect(plainText('Local files, Markdown and other `content`')).toBe(
      'Local files, Markdown and other content',
    );
    expect(plainText('**bold** and *it*')).toBe('bold and it');
    expect(plainText('a [link](https://x) here')).toBe('a link here');
  });
});

describe('leadParagraph', () => {
  it('takes the first line that is prose, skipping code', () => {
    expect(leadParagraph('```sh\ncurl https://x | sh\n```\n\nInstall it and read.')).toBe('Install it and read.');
  });

  it('drops list and heading markers', () => {
    expect(leadParagraph('- **first** item\n- second')).toBe('first item');
  });

  it('truncates a very long paragraph', () => {
    const text = leadParagraph('word '.repeat(120));
    expect(text.length).toBeLessThanOrEqual(241);
    expect(text.endsWith('…')).toBe(true);
  });

  it('returns nothing when there is no prose', () => {
    expect(leadParagraph('```\ncode\n```\n')).toBe('');
  });
});

describe('splitDocument', () => {
  const doc = splitDocument(guide);

  it('reads the document title', () => {
    expect(doc.title).toBe('wr guide');
  });

  it('summarizes with the first paragraph, skipping the manual TOC', () => {
    expect(doc.summary).toBe('How to read and browse the web from your terminal with wr.');
  });

  it('drops the manual table of contents from the intro', () => {
    expect(doc.introMarkdown).not.toContain('[Start](#start)');
    expect(doc.introMarkdown).toContain('How to read and browse');
  });

  it('splits at every level-2 heading, in order', () => {
    expect(doc.sections.map((s) => s.slug)).toEqual([
      'start',
      'the-screen',
      'local-files-markdown-and-other-content',
    ]);
    expect(doc.sections.map((s) => s.order)).toEqual([1, 2, 3]);
  });

  it('keeps fenced content out of the heading scan', () => {
    expect(doc.sections).toHaveLength(3);
    expect(doc.sections[1]!.markdown).toContain('## this fence is not a heading');
  });

  it('leaves the section body without its own heading', () => {
    expect(doc.sections[0]!.heading).toBe('Start');
    expect(doc.sections[0]!.markdown.startsWith('Install it:')).toBe(true);
  });
});
