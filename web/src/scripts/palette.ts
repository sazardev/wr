import { applyTheme } from './themes';

// The command palette (Ctrl+K or the app bar button): one dialog that lists
// every destination and action of the site, filtered as you type. The items are
// server-rendered markup; this module opens the dialog, filters it, moves the
// cursor and runs the chosen command.

let modal: HTMLElement | null = null;
let input: HTMLInputElement | null = null;
let list: HTMLElement | null = null;
let trigger: HTMLElement | null = null;
let options: HTMLElement[] = [];
let groups: HTMLElement[] = [];
let visible: HTMLElement[] = [];
let cursor = 0;

/** Subsequence match, and a score: the earlier and the tighter, the higher. */
function score(haystack: string, needle: string): number {
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

function paint(): void {
  if (!list) return;
  const query = (input?.value ?? '').trim().toLowerCase();

  const ranked = options
    .map((option, index) => {
      const hay = option.dataset.keys ? `${option.dataset.label} ${option.dataset.keys}` : option.dataset.label!;
      return { option, index, points: score(hay.toLowerCase(), query) };
    })
    .filter((entry) => entry.points > 0)
    .sort((a, b) => b.points - a.points || a.index - b.index);

  // What does not match is hidden; what does is re-appended in score order,
  // so the query decides what you see and in which order. With an empty query
  // every item scores the same and the original order comes back.
  const shown = new Set(ranked.map((entry) => entry.option));
  for (const option of options) {
    option.hidden = !shown.has(option);
    if (!shown.has(option)) option.setAttribute('aria-selected', 'false');
  }
  for (const group of groups) {
    const inside = ranked.filter((entry) => group.contains(entry.option));
    group.hidden = inside.length === 0;
    for (const entry of inside) group.appendChild(entry.option);
  }
  visible = ranked.map((entry) => entry.option);
  cursor = 0;
  syncCursor();
  const empty = list.querySelector<HTMLElement>('[data-palette-empty]');
  if (empty) empty.hidden = visible.length > 0;
}

function syncCursor(): void {
  visible.forEach((option, i) => {
    const on = i === cursor;
    option.setAttribute('aria-selected', String(on));
    if (on) {
      option.scrollIntoView({ block: 'nearest' });
      input?.setAttribute('aria-activedescendant', '');
    }
  });
}

function move(delta: number): void {
  if (!visible.length) return;
  cursor = (cursor + delta + visible.length) % visible.length;
  syncCursor();
}

function run(option: HTMLElement): void {
  close();
  if (option.dataset.action === 'theme') {
    applyTheme(option.dataset.theme!, true);
    return;
  }
  switch (option.dataset.action) {
    case 'copy':
      void navigator.clipboard?.writeText(location.href).catch(() => undefined);
      return;
    case 'top':
      scrollTo({ top: 0, behavior: 'smooth' });
      return;
    case 'print':
      print();
      return;
  }
  if (option.dataset.href) location.assign(option.dataset.href);
}

function open(): void {
  if (!modal || !input) return;
  modal.hidden = false;
  trigger?.setAttribute('aria-expanded', 'true');
  document.body.classList.add('palette-open');
  input.value = '';
  paint();
  input.focus();
}

export function close(): void {
  if (!modal || modal.hidden) return;
  modal.hidden = true;
  trigger?.setAttribute('aria-expanded', 'false');
  document.body.classList.remove('palette-open');
  trigger?.focus();
}

export function toggle(): void {
  if (!modal) return;
  if (modal.hidden) open();
  else close();
}

/** Wires the app bar trigger and the dialog it opens. */
export function startPalette(): void {
  const triggerEl = document.querySelector<HTMLElement>('[data-palette-open]');
  if (!triggerEl || triggerEl.dataset.wired === '1') return;
  const modalEl = document.querySelector<HTMLElement>('[data-palette]');
  const inputEl = document.querySelector<HTMLInputElement>('[data-palette-input]');
  const listEl = document.querySelector<HTMLElement>('[data-palette-list]');
  if (!modalEl || !inputEl || !listEl) return;

  triggerEl.dataset.wired = '1';
  modal = modalEl;
  input = inputEl;
  list = listEl;
  trigger = triggerEl;
  options = Array.from(list.querySelectorAll<HTMLElement>('[data-palette-option]'));
  groups = Array.from(list.querySelectorAll<HTMLElement>('[data-palette-group]'));

  // The "nothing matches" row lives with the items and is only ever unhidden.
  const empty = document.createElement('div');
  empty.className = 'pal-empty';
  empty.dataset.paletteEmpty = '1';
  empty.hidden = true;
  empty.innerHTML = '<span class="glyph">⠿</span> nothing matches that';
  list.append(empty);

  trigger.addEventListener('click', () => toggle());
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
      case 'Tab':
        event.preventDefault();
        move(event.shiftKey ? -1 : 1);
        break;
      case 'Home':
        event.preventDefault();
        cursor = 0;
        syncCursor();
        break;
      case 'End':
        event.preventDefault();
        cursor = visible.length - 1;
        syncCursor();
        break;
    }
  });

  list.addEventListener('click', (event) => {
    const option = (event.target as HTMLElement).closest<HTMLElement>('[data-palette-option]');
    if (option) run(option);
  });
  list.addEventListener('pointermove', (event) => {
    const option = (event.target as HTMLElement).closest<HTMLElement>('[data-palette-option]');
    const index = option ? visible.indexOf(option) : -1;
    if (index >= 0 && index !== cursor) {
      cursor = index;
      syncCursor();
    }
  });
}
