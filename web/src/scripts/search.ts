import { searchEntries, tokenize, type SearchEntry } from '../lib/search';
import { withBase } from '../lib/urls';
import { go } from './go';

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
  let selected = -1;

  const rows = (): HTMLAnchorElement[] => [...search.results.querySelectorAll<HTMLAnchorElement>('a.result')];
  const select = (next: number): void => {
    const list = rows();
    if (!list.length) return;
    selected = (next + list.length) % list.length;
    list.forEach((row, i) => row.classList.toggle('sel', i === selected));
    list[selected]!.scrollIntoView({ block: 'nearest' });
  };

  const entries = await loadIndex();

  const emptyState = (lead: string, hint: string): HTMLDivElement => {
    const box = document.createElement('div');
    box.className = 'empty';
    const title = document.createElement('b');
    title.textContent = lead;
    const note = document.createElement('span');
    note.textContent = hint;
    box.append(title, note);
    return box;
  };

  const render = (): void => {
    const terms = tokenize(query);
    const hits = searchEntries(entries, query, { scope });
    search.results.textContent = '';

    if (!query.trim()) {
      search.status.textContent = `${entries.length} pages indexed`;
      search.results.append(emptyState('Search the docs', 'Type a word: links, keys, cache, install…'));
      return;
    }
    search.status.textContent = hits.length === 1 ? '1 result' : `${hits.length} results`;

    if (hits.length === 0) {
      search.results.append(emptyState(`Nothing matches “${query}”.`, 'Try a single word: links, keys, cache, install…'));
      selected = -1;
      return;
    }
    for (const hit of hits) search.results.append(hitElement(hit, terms));
    selected = -1;
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

  search.input.addEventListener('keydown', (event) => {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      select(selected + 1);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      select(selected - 1);
    }
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
    const list = rows();
    const target = list[selected] ?? list[0];
    event.preventDefault();
    if (target) go(target.href);
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
  const link = document.createElement('a');
  link.className = 'result';
  link.href = hit.entry.url;

  const where = document.createElement('div');
  where.className = 'where';
  const doc = document.createElement('span');
  doc.className = 'doc';
  doc.textContent = hit.entry.doc;
  where.append(doc);

  const heading = document.createElement('h2');
  heading.textContent = hit.entry.title;

  const body = document.createElement('p');
  body.append(highlight(hit.snippet, terms));

  link.append(where, heading, body);
  return link;
}
