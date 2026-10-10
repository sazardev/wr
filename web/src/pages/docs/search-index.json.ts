import type { APIRoute } from 'astro';
import { getCollection } from 'astro:content';
import { shortcutGroups, settingRows, pathRows } from '../../lib/reference';
import type { SearchEntry } from '../../lib/search';
import { docPath, withBase } from '../../lib/urls';

// The search index is a static file built from the collection and from the
// reference generators: the search page fetches it and filters in the browser.
// No service, no index to keep in sync, nothing to deploy but the pages.

function referenceEntries(): SearchEntry[] {
  const url = withBase('/docs/reference/');
  const shortcuts: SearchEntry = {
    doc: 'reference',
    section: 'shortcuts',
    title: 'Shortcuts',
    url: withBase('/docs/reference/shortcuts/'),
    text: shortcutGroups()
      .map((group) => `${group.group}: ${group.items.map((item) => `${item.label} (${item.keys.join(', ')})`).join('; ')}`)
      .join('. '),
  };
  const settings: SearchEntry = {
    doc: 'reference',
    section: 'settings',
    title: 'Settings',
    url: withBase('/docs/reference/settings/'),
    text: settingRows().map((row) => `${row.key} (${row.default}): ${row.description}`).join('. '),
  };
  const paths: SearchEntry = {
    doc: 'reference',
    section: 'paths',
    title: 'Where wr keeps things',
    url,
    text: pathRows().map((row) => `${row.path}: ${row.what}`).join('. '),
  };
  return [shortcuts, settings, paths];
}

export const GET: APIRoute = async () => {
  const docs = await getCollection('docs');
  const index: SearchEntry[] = docs
    .filter((entry) => entry.data.text.trim().length > 0)
    .map((entry) => ({
      doc: entry.data.doc,
      section: entry.data.section,
      title: entry.data.title,
      url: docPath(entry.data.doc, entry.data.section),
      text: entry.data.text,
    }));
  index.push(...referenceEntries());

  return new Response(JSON.stringify(index), {
    headers: { 'content-type': 'application/json; charset=utf-8' },
  });
};
