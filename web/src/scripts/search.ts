import { searchEntries, tokenize, type SearchEntry } from '../lib/search';
import { withBase } from '../lib/urls';

// The search page: one static page whose state lives in the query string, so a
// search is a link that can be shared, bookmarked and gone back to. The index
// is a JSON file generated at build time from the same sections the pages are
// made of, so no search service is involved.

const INDEX_URL = withBase('/docs/search-index.json');
const SCOPES = ['all', 'guide', 'design', 'changelog', 'reference'];

interface SearchView {
  input: HTMLInputElement;
  form: HTMLFormElement;
  status: HTMLElement;
  results: HTMLElement;
  scopes: HTMLButtonElement[];
}

function view(): SearchView | null {
  const root = document.getElementById('search');
  if (!root || root.dataset.ready === '1') return null;
  const input = root.querySelector<HTMLInputElement>('[data-search-input]');
  const form = root.querySelector<HTMLFormElement>('[data-search-form]');
  const status = root.querySelector<HTMLElement>('[data-search-status]');
  const results = root.querySelector<HTMLElement>('[data-search-results]');
  if (!input || !form || !status || !results) return null;
  root.dataset.ready = '1';
  return {
    input,
    form,
    status,
    results,
    scopes: [...root.querySelectorAll<HTMLButtonElement>('[data-scope]')],
  };
}

async function loadIndex(): Promise<SearchEntry[]> {
  try {
    const response = await fetch(INDEX_URL);
    if (!response.ok) return [];
    return (await response.json()) as SearchEntry[];
  } catch {
    return [];
  }
}

function escapeRegExp(source: string): string {
  return source.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/** Wraps every match of the query words in a <mark>, without innerHTML. */
function highlight(text: string, terms: string[]): DocumentFragment {
  const fragment = document.createDocumentFragment();
  if (terms.length === 0) {
    fragment.append(text);
    return fragment;
  }
  const wanted = new Set(terms.map((term) => term.toLowerCase()));
  for (const part of text.split(new RegExp(`(${terms.map(escapeRegExp).join('|')})`, 'gi'))) {
    if (!part) continue;
    if (wanted.has(part.toLowerCase())) {
      const mark = document.createElement('mark');
      mark.textContent = part;
      fragment.append(mark);
    } else {
      fragment.append(part);
    }
  }
  return fragment;
}

export async function startSearch(): Promise<void> {
  const search = view();
  if (!search) return;

  const initial = new URLSearchParams(location.search);
  let query = initial.get('q') ?? '';
  let scope = SCOPES.includes(initial.get('scope') ?? '') ? initial.get('scope')! : 'all';
  search.input.value = query;

  const entries = await loadIndex();

  const render = (): void => {
    const terms = tokenize(query);
    const hits = searchEntries(entries, query, { scope });
    search.results.textContent = '';

    if (!query.trim()) {
      search.status.textContent = `${entries.length} pages indexed`;
      return;
    }
    search.status.textContent = hits.length === 1 ? '1 result' : `${hits.length} results`;

    if (hits.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'empty';
      const glyph = document.createElement('span');
      glyph.className = 'glyph';
      glyph.textContent = '⠿';
      const text = document.createElement('p');
      text.textContent = `Nothing matches “${query}”. Try a single word: links, keys, cache, install…`;
      empty.append(glyph, text);
      search.results.append(empty);
      return;
    }
    for (const hit of hits) search.results.append(hitElement(hit, terms));
  };

  const syncUrl = (): void => {
    const next = new URLSearchParams();
    if (query.trim()) next.set('q', query);
    if (scope !== 'all') next.set('scope', scope);
    const search_ = next.toString();
    history.replaceState(null, '', `${location.pathname}${search_ ? `?${search_}` : ''}`);
  };

  search.input.addEventListener('input', () => {
    query = search.input.value;
    syncUrl();
    render();
  });

  for (const button of search.scopes) {
    button.setAttribute('aria-pressed', String(button.dataset.scope === scope));
    button.addEventListener('click', () => {
      scope = button.dataset.scope!;
      for (const other of search.scopes) other.setAttribute('aria-pressed', String(other === button));
      syncUrl();
      render();
    });
  }

  search.form.addEventListener('submit', (event) => {
    const [first] = searchEntries(entries, query, { scope });
    if (first) {
      event.preventDefault();
      location.assign(first.entry.url);
    }
  });

  addEventListener('popstate', () => {
    const next = new URLSearchParams(location.search);
    query = next.get('q') ?? '';
    scope = next.get('scope') ?? 'all';
    search.input.value = query;
    for (const button of search.scopes) {
      button.setAttribute('aria-pressed', String(button.dataset.scope === scope));
    }
    render();
  });

  render();
}

function hitElement(hit: { entry: SearchEntry; snippet: string }, terms: string[]): HTMLElement {
  const article = document.createElement('article');
  article.className = 'result';

  const where = document.createElement('div');
  where.className = 'where';
  const doc = document.createElement('span');
  doc.className = 'doc';
  doc.textContent = hit.entry.doc.toUpperCase();
  where.append(doc);

  const heading = document.createElement('h3');
  const link = document.createElement('a');
  link.href = hit.entry.url;
  link.textContent = hit.entry.title;
  heading.append(link);

  const body = document.createElement('p');
  body.append(highlight(hit.snippet, terms));

  article.append(where, heading, body);
  return article;
}
