import { searchEntries, type SearchEntry } from '../lib/search';
import { withBase } from '../lib/urls';
import { go } from './go';
import { openHelp } from './help';
import { applyTheme } from './themes';

// The command palette (Ctrl+K, "/" or the header button). One input over two
// kinds of rows:
//   · the commands in the markup — pages, guide sections, themes, actions —
//     filtered by a fuzzy subsequence match as you type;
//   · the full-text hits of the search index, fetched the first time the
//     palette opens, shown above the commands once the query has two letters.
// The cursor moves with the arrows, Enter runs the row, Esc gives the page back.

const INDEX_URL = withBase('/docs/search-index.json');
const MAX_HITS = 6;

let modal: HTMLElement | null = null;
let input: HTMLInputElement | null = null;
let list: HTMLElement | null = null;
let hitsGroup: HTMLElement | null = null;
let options: HTMLElement[] = [];
let groups: HTMLElement[] = [];
let visible: HTMLElement[] = [];
let opener: HTMLElement | null = null;
let cursor = 0;

let index: SearchEntry[] | null = null;
let loading = false;

/** Subsequence match, and a score: the earlier and the tighter, the higher. */
export function score(haystack: string, needle: string): number {
  if (!needle) return 1;
  let at = 0;
  let points = 0;
  let streak = 0;
  for (const char of needle) {
    const found = haystack.indexOf(char, at);
    if (found < 0) return 0;
    streak = found === at ? streak + 1 : 0;
    points += (found === 0 ? 12 : 0) + (haystack[found - 1] === ' ' ? 6 : 0) + streak;
    at = found + 1;
  }
  return points + Math.max(0, 12 - haystack.length / 8);
}

async function loadIndex(): Promise<void> {
  if (index || loading) return;
  loading = true;
  try {
    const response = await fetch(INDEX_URL);
    index = response.ok ? ((await response.json()) as SearchEntry[]) : [];
  } catch {
    index = [];
  }
  loading = false;
  if (isOpen()) paint();
}

function escapeRegExp(source: string): string {
  return source.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function highlight(text: string, words: string[]): DocumentFragment {
  const fragment = document.createDocumentFragment();
  if (!words.length) {
    fragment.append(text);
    return fragment;
  }
  const pattern = new RegExp(`(${words.map(escapeRegExp).join('|')})`, 'gi');
  for (const part of text.split(pattern)) {
    if (!part) continue;
    if (words.includes(part.toLowerCase())) {
      const mark = document.createElement('mark');
      mark.textContent = part;
      fragment.append(mark);
    } else fragment.append(part);
  }
  return fragment;
}

function hitRow(label: string, where: string, href: string, snippet?: string, words: string[] = []): HTMLElement {
  const row = document.createElement('div');
  row.className = 'pal-opt';
  row.setAttribute('role', 'option');
  row.setAttribute('aria-selected', 'false');
  row.dataset.href = href;
  const mark = document.createElement('span');
  mark.className = 'mark';
  mark.textContent = '▸';
  mark.setAttribute('aria-hidden', 'true');
  const col = document.createElement('span');
  col.className = 'col';
  const name = document.createElement('span');
  name.className = 'nm';
  name.append(highlight(label, words));
  col.append(name);
  if (snippet) {
    const snip = document.createElement('span');
    snip.className = 'snip';
    snip.append(highlight(snippet, words));
    col.append(snip);
  }
  const tag = document.createElement('span');
  tag.className = 'where';
  tag.textContent = where;
  row.append(mark, col, tag);
  return row;
}

function isOpen(): boolean {
  return Boolean(modal && !modal.hidden);
}

function paint(): void {
  if (!list || !hitsGroup) return;
  const query = (input?.value ?? '').trim().toLowerCase();

  // the commands
  const ranked = options
    .map((option, at) => {
      const hay = `${option.dataset.label} ${option.dataset.keys ?? ''}`.toLowerCase();
      return { option, at, points: score(hay, query) };
    })
    .filter((entry) => entry.points > 0)
    .sort((a, b) => b.points - a.points || a.at - b.at);
  const shown = new Set(ranked.map((entry) => entry.option));
  for (const option of options) option.hidden = !shown.has(option);
  for (const group of groups) {
    const inside = ranked.filter((entry) => group.contains(entry.option));
    group.hidden = inside.length === 0;
    for (const entry of inside) group.appendChild(entry.option);
  }

  // the full-text hits
  const words = query.split(/[^\p{L}\p{N}]+/u).filter((word) => word.length > 1);
  hitsGroup.querySelectorAll('.pal-opt').forEach((row) => row.remove());
  let hits: HTMLElement[] = [];
  if (index && words.length) {
    hits = searchEntries(index, query, { limit: MAX_HITS }).map((hit) =>
      hitRow(hit.entry.title, hit.entry.doc, hit.entry.url, hit.snippet, words),
    );
    if (hits.length) {
      hits.push(hitRow(`See every result for “${input!.value.trim()}”`, 'search', `${withBase('/docs/search/')}?q=${encodeURIComponent(input!.value.trim())}`));
    }
  }
  hitsGroup.hidden = hits.length === 0;
  for (const row of hits) hitsGroup.append(row);

  visible = [...hits, ...ranked.map((entry) => entry.option)];
  visible.forEach((row, i) => (row.id = `pal-o-${i}`));
  cursor = 0;
  syncCursor();

  let empty = list.querySelector<HTMLElement>('[data-palette-empty]');
  if (!empty) {
    empty = document.createElement('div');
    empty.className = 'pal-empty';
    empty.dataset.paletteEmpty = '1';
    list.append(empty);
  }
  empty.textContent = index === null && words.length ? 'Searching the docs…' : `Nothing matches “${input?.value.trim()}”.`;
  empty.hidden = visible.length > 0;
}

function syncCursor(): void {
  visible.forEach((row, i) => {
    const on = i === cursor;
    row.setAttribute('aria-selected', String(on));
    if (on) {
      row.scrollIntoView({ block: 'nearest' });
      input?.setAttribute('aria-activedescendant', row.id);
    }
  });
}

function move(delta: number): void {
  if (!visible.length) return;
  cursor = (cursor + delta + visible.length) % visible.length;
  syncCursor();
}

function run(row: HTMLElement): void {
  close(false);
  switch (row.dataset.action) {
    case 'theme':
      applyTheme(row.dataset.theme!, true);
      return;
    case 'copy':
      void navigator.clipboard?.writeText(location.href).catch(() => undefined);
      return;
    case 'top':
      scrollTo({ top: 0, behavior: 'smooth' });
      return;
    case 'print':
      print();
      return;
    case 'help':
      openHelp();
      return;
  }
  if (row.dataset.href) go(row.dataset.href);
}

export function open(): void {
  if (!modal || !input) return;
  opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  modal.hidden = false;
  document.querySelectorAll('[data-palette-open]').forEach((el) => el.setAttribute('aria-expanded', 'true'));
  document.body.classList.add('lock');
  input.value = '';
  paint();
  input.focus();
  void loadIndex();
}

export function close(restoreFocus = true): void {
  if (!modal || modal.hidden) return;
  modal.hidden = true;
  document.querySelectorAll('[data-palette-open]').forEach((el) => el.setAttribute('aria-expanded', 'false'));
  document.body.classList.remove('lock');
  if (restoreFocus) opener?.focus();
}

export function toggle(): void {
  if (isOpen()) close();
  else open();
}

export function startPalette(): void {
  const modalEl = document.querySelector<HTMLElement>('[data-palette]');
  const inputEl = document.querySelector<HTMLInputElement>('[data-palette-input]');
  const listEl = document.querySelector<HTMLElement>('[data-palette-list]');
  if (!modalEl || !inputEl || !listEl) return;

  document.querySelectorAll<HTMLElement>('[data-palette-open]').forEach((button) => {
    if (button.dataset.wired === '1') return;
    button.dataset.wired = '1';
    button.addEventListener('click', () => open());
  });
  if (modalEl.dataset.wired === '1') return;
  modalEl.dataset.wired = '1';

  modal = modalEl;
  input = inputEl;
  list = listEl;
  hitsGroup = list.querySelector<HTMLElement>('[data-palette-hits]');
  options = [...list.querySelectorAll<HTMLElement>('[data-palette-option]')];
  groups = [...list.querySelectorAll<HTMLElement>('[data-palette-group]')];

  modal.addEventListener('click', (event) => {
    if (event.target === modal) close();
  });
  input.addEventListener('input', paint);
  input.addEventListener('keydown', (event) => {
    switch (event.key) {
      case 'Escape':
        event.preventDefault();
        close();
        break;
      case 'Enter':
        event.preventDefault();
        if (visible[cursor]) run(visible[cursor]!);
        break;
      case 'ArrowDown':
        event.preventDefault();
        move(1);
        break;
      case 'ArrowUp':
        event.preventDefault();
        move(-1);
        break;
      case 'n':
      case 'j':
        if (event.ctrlKey) {
          event.preventDefault();
          move(1);
        }
        break;
      case 'p':
      case 'k':
        if (event.ctrlKey) {
          event.preventDefault();
          move(-1);
        }
        break;
      case 'Tab':
        event.preventDefault();
        move(event.shiftKey ? -1 : 1);
        break;
      case 'Home':
        if (!input!.value) {
          event.preventDefault();
          cursor = 0;
          syncCursor();
        }
        break;
      case 'End':
        if (!input!.value) {
          event.preventDefault();
          cursor = visible.length - 1;
          syncCursor();
        }
        break;
    }
  });
  list.addEventListener('click', (event) => {
    const row = (event.target as HTMLElement).closest<HTMLElement>('.pal-opt');
    if (row && visible.includes(row)) run(row);
  });
  list.addEventListener('pointermove', (event) => {
    const row = (event.target as HTMLElement).closest<HTMLElement>('.pal-opt');
    const at = row ? visible.indexOf(row) : -1;
    if (at >= 0 && at !== cursor) {
      cursor = at;
      visible.forEach((other, i) => other.setAttribute('aria-selected', String(i === at)));
    }
  });
}
