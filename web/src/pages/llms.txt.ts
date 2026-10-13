import type { APIRoute } from 'astro';
import { docEntries } from '../lib/doc-nav';
import { DOCS } from '../lib/registry';
import { withBase } from '../lib/urls';

// /llms.txt — the map an LLM reads first (https://llmstxt.org): what wr is,
// then one line per documentation page. It is built from the same registry and
// collection as the site, so it cannot drift from the real pages.

export const GET: APIRoute = async ({ site }) => {
  const origin = site ?? new URL('https://sazardev.github.io');
  const url = (path: string) => new URL(withBase(path), origin).toString();

  const docs = [...DOCS].sort((a, b) => a.order - b.order);
  const sections: string[] = [];
  for (const doc of docs) {
    const entries = (await docEntries(doc.id)).filter((entry) => entry.data.section !== 'index');
    const lines = entries.map(
      (entry) => `- [${entry.data.title}](${url(`/docs/${doc.id}/${entry.data.section}/`)}): ${entry.data.summary || doc.blurb}`,
    );
    sections.push(`## ${doc.label}\n\n${doc.blurb}\n\n${lines.join('\n')}`);
  }

  const body = [
    '# wr',
    '',
    '> wr is a reader and browser for the terminal: it fetches a page, extracts the article and renders it as formatted Markdown with per-language code highlighting, links, history, bookmarks and search. One static binary, MIT, Linux and macOS.',
    '',
    "wr is a Go program (built on Bubble Tea) that turns any web page into clean, typeset text. It drops menus, banners and popups; renders headings, lists, tables and code; opens the links it finds; keeps history and bookmarks; and lets the reader change every setting and key binding from inside the program. It wears the terminal's own 16-color palette, so it follows the user's theme with zero configuration. Pages that only exist once their JavaScript runs are rendered through an installed headless Chromium.",
    '',
    'Install: `curl -fsSL https://raw.githubusercontent.com/sazardev/wr/main/install.sh | sh` — or `go install github.com/sazardev/wr@latest`. Releases and checksums: https://github.com/sazardev/wr/releases',
    '',
    ...sections.map((section) => `${section}\n`),
    '## Reference',
    '',
    `- [All shortcuts](${url('/docs/reference/shortcuts/')}): every default key binding, generated from the source`,
    `- [All settings](${url('/docs/reference/settings/')}): every option, with its default and its help text`,
    `- [Search](${url('/docs/search/')}): full-text search over every page of the documentation`,
    '',
    '## Optional',
    '',
    `- [Changelog](${url('/docs/changelog/')}): every release, written from the repository history`,
    '- [Source on GitHub](https://github.com/sazardev/wr): code, issues, releases — MIT licensed',
    `- [Full documentation in one file](${url('/llms-full.txt')}): README, guide, design language and changelog concatenated`,
    '',
  ].join('\n');

  return new Response(body, { headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
};
