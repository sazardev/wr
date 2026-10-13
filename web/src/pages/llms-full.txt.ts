import type { APIRoute } from 'astro';
import { readChangelog, readDesign, readGuide, readReadme } from '../lib/repo';

// /llms-full.txt — the whole written documentation in one plain file, for
// assistants that prefer one fetch over crawling (https://llmstxt.org). The
// pieces are the repository's own Markdown, in reading order.

export const GET: APIRoute = () => {
  const body = [
    '# wr — full documentation',
    '',
    '> The complete documentation of wr in one file: README, guide, design language and changelog, as written in the repository. Relative links and image paths resolve against https://github.com/sazardev/wr/blob/main/',
    '',
    readReadme(),
    readGuide(),
    readDesign(),
    readChangelog(),
  ].join('\n\n---\n\n');

  return new Response(body, { headers: { 'Content-Type': 'text/plain; charset=utf-8' } });
};
