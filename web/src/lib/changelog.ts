// The changelog page is generated from CHANGELOG.md, the file release-please
// writes on every release: the docs site publishes it as it is written, one
// deep route per release.

export interface Release {
  version: string;
  /** Compare URL, when release-please linked one. */
  url: string | null;
  /** ISO date, when release-please stamped one. */
  date: string | null;
  /** Release notes in Markdown (the ### sections and the list items). */
  markdown: string;
  /** Plain text of the notes, for the search index. */
  text: string;
  /** True for the newest entry, which the bare `/docs/changelog/` opens. */
  latest: boolean;
}

export interface Changelog {
  title: string;
  intro: string;
  releases: Release[];
}

const RELEASE = /^##\s+(?:\[([^\]]+)]\(([^)]*)\)|v?([^\s(]+))\s*(?:\(([^)]*)\))?\s*$/;

/** Parses a release-please changelog into releases. */
export function parseChangelog(markdown: string): Changelog {
  const lines = markdown.split('\n');
  const title = lines.find((l) => /^#\s+/.test(l))?.replace(/^#\s+/, '').trim() ?? 'Changelog';

  const blocks: { version: string; url: string | null; date: string | null; body: string[] }[] = [];
  let current: { version: string; url: string | null; date: string | null; body: string[] } | null = null;

  for (const line of lines) {
    const match = line.match(RELEASE);
    if (match) {
      current = {
        version: match[1] ?? match[3] ?? '',
        url: match[2] ?? null,
        date: match[4] ?? null,
        body: [],
      };
      blocks.push(current);
      continue;
    }
    if (current) current.body.push(line);
  }

  const intro = current ? '' : lines.filter((l) => !/^#/.test(l)).join('\n').trim();
  const releases = blocks.map((block, i) => ({
    version: block.version.replace(/^v/, ''),
    url: block.url,
    date: block.date,
    markdown: block.body.join('\n').trim(),
    text: block.body
      .join(' ')
      .replace(/\[([^\]]*)]\([^)]*\)/g, '$1')
      .replace(/[#*`]/g, ' ')
      .replace(/\s+/g, ' ')
      .trim(),
    latest: i === 0,
  }));

  return { title, intro, releases };
}
