import { DEFAULT_THEME, STORAGE_KEY, THEMES, themeById } from '../data/themes';

// Themes: the visitor picks a palette, the choice is remembered, and a
// ?theme= link opens the site already dressed. Server-rendered chips carry
// data-theme-pick; this module only applies the choice and keeps the pressed
// states honest. Every other module listens for the `wr-theme` event.

const VALID = new Set(THEMES.map((theme) => theme.id));
const EVENT = 'wr-theme';

let current = DEFAULT_THEME;

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
