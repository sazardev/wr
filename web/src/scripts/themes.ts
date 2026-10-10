import { DEFAULT_THEME, STORAGE_KEY, THEMES, themeById } from '../data/themes';

// Themes: the visitor opens the theme dialog in the app bar, walks the palettes
// with the arrow keys and applies one with Enter. The choice is remembered, and
// a ?theme= link opens the site already dressed. The dialog previews a palette
// as the cursor moves over it; Esc puts back the one that was active when the
// dialog opened. Server-rendered swatches carry data-theme-pick; this module
// applies the choice, keeps the pressed states honest and drives the dialog.
// Every other module listens for the `wr-theme` event.

const VALID = new Set(THEMES.map((theme) => theme.id));
const EVENT = 'wr-theme';

let current = DEFAULT_THEME;

// The dialog, its listbox and the trigger, once the document has them.
let modal: HTMLElement | null = null;
let list: HTMLElement | null = null;
let trigger: HTMLElement | null = null;
let options: HTMLElement[] = [];
let cursor = 0;
let openedWith = DEFAULT_THEME;

export function applyTheme(id: string, persist = true): string {
  const theme = VALID.has(id) ? id : DEFAULT_THEME;
  current = theme;
  document.documentElement.dataset.theme = theme;
  if (persist) {
    try {
      localStorage.setItem(STORAGE_KEY, theme);
    } catch {
      /* private mode: the theme just does not survive a reload */
    }
  }
  syncPicked();
  syncTrigger();
  document.dispatchEvent(new CustomEvent(EVENT, { detail: { id: theme } }));
  return theme;
}

export function currentTheme(): string {
  return current;
}

function syncPicked(): void {
  document.querySelectorAll<HTMLElement>('[data-theme-pick]').forEach((el) => {
    el.setAttribute('aria-pressed', String(el.dataset.themePick === current));
  });
}

/** The theme the visitor should see first: the link wins, then memory, then the system. */
export function initialTheme(search = location.search): string {
  const fromUrl = new URLSearchParams(search).get('theme');
  if (fromUrl && VALID.has(fromUrl)) return fromUrl;
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored && VALID.has(stored)) return stored;
  } catch {
    /* no storage: fall through to the system preference */
  }
  return matchMedia('(prefers-color-scheme: light)').matches ? 'paper' : 'wr';
}

/** Applies the initial theme and wires every chip on the page. */
export function startThemes(): void {
  applyTheme(initialTheme(), false);
  document.querySelectorAll<HTMLElement>('[data-theme-pick]').forEach((el) => {
    if (el.dataset.wired === '1') return;
    el.dataset.wired = '1';
    el.addEventListener('click', () => applyTheme(el.dataset.themePick!, true));
  });
  syncPicked();
}

/** Palette of a theme, for scripts that need the color (not the CSS token). */
export function themeColor(id: string): string {
  return themeById(id).colors[4]!;
}

/* --------------------------------------------------------- theme dialog */

function syncTrigger(): void {
  const theme = themeById(current);
  const name = document.querySelector<HTMLElement>('[data-theme-name]');
  const scheme = document.querySelector<HTMLElement>('[data-theme-scheme]');
  const chips = document.querySelector<HTMLElement>('[data-theme-chips]');
  if (name) name.textContent = theme.id;
  if (scheme) scheme.textContent = theme.scheme;
  if (chips) {
    chips.innerHTML = [0, 1, 2, 4].map((i) => `<i style="background:${theme.colors[i]}"></i>`).join('');
  }
  if (trigger) {
    const label = `theme: ${theme.id} (${theme.scheme})`;
    trigger.title = label;
    trigger.setAttribute('aria-label', label);
  }
}

function syncMenu(): void {
  options.forEach((opt, i) => {
    const on = i === cursor;
    opt.setAttribute('aria-selected', String(on));
    opt.dataset.selected = on ? '1' : '0';
  });
  options[cursor]?.scrollIntoView({ block: 'nearest' });
}

function moveTo(index: number): void {
  if (index < 0 || index >= options.length) return;
  cursor = index;
  applyTheme(THEMES[index]!.id, false);
  syncMenu();
}

function openMenu(): void {
  if (!modal || !list) return;
  openedWith = current;
  cursor = Math.max(
    0,
    THEMES.findIndex((theme) => theme.id === current),
  );
  modal.hidden = false;
  trigger?.setAttribute('aria-expanded', 'true');
  document.body.classList.add('menu-open');
  syncMenu();
  list.focus();
}

function closeMenu(apply: boolean): void {
  if (!modal || modal.hidden) return;
  modal.hidden = true;
  trigger?.setAttribute('aria-expanded', 'false');
  document.body.classList.remove('menu-open');
  applyTheme(apply ? THEMES[cursor]!.id : openedWith, true);
  trigger?.focus();
}

function onMenuKey(event: KeyboardEvent): void {
  const n = options.length;
  switch (event.key) {
    case 'Escape':
      event.preventDefault();
      closeMenu(false);
      break;
    case 'Enter':
    case ' ':
      event.preventDefault();
      closeMenu(true);
      break;
    case 'ArrowDown':
      event.preventDefault();
      moveTo((cursor + 1) % n);
      break;
    case 'ArrowUp':
      event.preventDefault();
      moveTo((cursor - 1 + n) % n);
      break;
    case 'Home':
      event.preventDefault();
      moveTo(0);
      break;
    case 'End':
      event.preventDefault();
      moveTo(n - 1);
      break;
    case 'Tab':
      event.preventDefault();
      break;
  }
}

/** Wires the app bar trigger and the dialog it opens. */
export function startThemeMenu(): void {
  const modalEl = document.querySelector<HTMLElement>('[data-theme-modal]');
  const listEl = document.querySelector<HTMLElement>('[data-theme-list]');
  const triggerEl = document.querySelector<HTMLElement>('[data-theme-menu]');
  if (!modalEl || !listEl || !triggerEl || triggerEl.dataset.wired === '1') return;
  triggerEl.dataset.wired = '1';

  modal = modalEl;
  list = listEl;
  trigger = triggerEl;
  options = Array.from(list.querySelectorAll<HTMLElement>('[data-theme-option]'));

  trigger.addEventListener('click', () => (modal!.hidden ? openMenu() : closeMenu(false)));
  modal.addEventListener('click', (event) => {
    if (event.target === modal) closeMenu(false);
  });
  list.addEventListener('keydown', onMenuKey);
  list.addEventListener('click', (event) => {
    const opt = (event.target as HTMLElement).closest<HTMLElement>('[data-theme-option]');
    if (!opt) return;
    moveTo(options.indexOf(opt));
    closeMenu(true);
  });
  list.addEventListener('pointermove', (event) => {
    const opt = (event.target as HTMLElement).closest<HTMLElement>('[data-theme-option]');
    const index = opt ? options.indexOf(opt) : -1;
    if (index >= 0 && index !== cursor) moveTo(index);
  });
  syncTrigger();
}
