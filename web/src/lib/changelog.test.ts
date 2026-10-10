import { describe, expect, it } from 'vitest';
import { parseChangelog } from './changelog';

const changelog = `# Changelog

## [0.3.0](https://github.com/sazardev/wr/compare/v0.2.1...v0.3.0) (2026-10-09)

### Features

* give --help the reader's look ([f4df27d](https://github.com/sazardev/wr/commit/f4df27d))

## v0.1.0

First tagged release.

- Reader: formatted Markdown.
`;

describe('parseChangelog', () => {
  const parsed = parseChangelog(changelog);

  it('reads the newest release first', () => {
    expect(parsed.releases).toHaveLength(2);
    expect(parsed.releases[0]).toMatchObject({
      version: '0.3.0',
      url: 'https://github.com/sazardev/wr/compare/v0.2.1...v0.3.0',
      date: '2026-10-09',
      latest: true,
    });
  });

  it('accepts a plain version heading', () => {
    expect(parsed.releases[1]).toMatchObject({ version: '0.1.0', latest: false, date: null, url: null });
  });

  it('keeps the release notes and drops the link syntax from the text', () => {
    expect(parsed.releases[0]!.markdown).toContain('### Features');
    expect(parsed.releases[0]!.text).toContain("give --help the reader's look");
    expect(parsed.releases[0]!.text).not.toContain('https://');
    expect(parsed.releases[1]!.text).toContain('First tagged release.');
  });
});
