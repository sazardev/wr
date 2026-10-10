// Search runs over an index generated at build time from the same sections
// the pages are made of. The scoring is deliberately small and testable: words
// in a title weigh more than words in the body, and a document only matches
// when it contains every word.

export interface SearchEntry {
  /** Document id: guide, design, changelog, reference. */
  doc: string;
  /** Section slug, or 'index' for the document intro. */
  section: string;
  title: string;
  url: string;
  /** Plain text of the page. */
  text: string;
}

export interface SearchHit {
  entry: SearchEntry;
  score: number;
  /** Window of the body around the first match. */
  snippet: string;
}

const SNIPPET = 220;

/** Words of a query: anything that is not a letter or a number splits. */
export function tokenize(query: string): string[] {
  return query
    .toLowerCase()
    .split(/[^\p{L}\p{N}]+/u)
    .filter((t) => t.length > 1);
}

/** Ranks entries for a query; an entry must contain every word. */
export function searchEntries(
  entries: readonly SearchEntry[],
  query: string,
  options: { scope?: string; limit?: number } = {},
): SearchHit[] {
  const terms = tokenize(query);
  const limit = options.limit ?? 24;
  if (terms.length === 0) return [];

  const hits: SearchHit[] = [];
  for (const entry of entries) {
    if (options.scope && options.scope !== 'all' && entry.doc !== options.scope) continue;
    const title = entry.title.toLowerCase();
    const body = entry.text.toLowerCase();
    let score = 0;
    let matched = true;
    for (const term of terms) {
      const inTitle = title.includes(term) ? 1 : 0;
      const occurrences = countOccurrences(body, term);
      if (!inTitle && occurrences === 0) {
        matched = false;
        break;
      }
      score += inTitle * 8 + Math.min(occurrences, 12) + (body.startsWith(term) ? 2 : 0);
    }
    if (!matched) continue;
    hits.push({ entry, score, snippet: snippetFor(entry.text, terms[0]!) });
  }

  return hits.sort((a, b) => b.score - a.score || a.entry.title.localeCompare(b.entry.title)).slice(0, limit);
}

function countOccurrences(haystack: string, needle: string): number {
  let count = 0;
  let from = 0;
  for (;;) {
    const at = haystack.indexOf(needle, from);
    if (at === -1) return count;
    count++;
    from = at + needle.length;
  }
}

/** Window of the text around its first match, cut on word boundaries. */
export function snippetFor(text: string, term: string): string {
  const at = text.toLowerCase().indexOf(term.toLowerCase());
  if (at === -1) return text.slice(0, SNIPPET).trim();
  const start = Math.max(0, at - Math.floor(SNIPPET / 3));
  const end = Math.min(text.length, start + SNIPPET);
  const cut = (text.slice(start, end) + (end < text.length ? '…' : '')).trim();
  return start > 0 ? `…${cut}` : cut;
}
