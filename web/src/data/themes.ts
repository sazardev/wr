// Theme data, shared by the server (which renders the chips) and the client
// script (which applies and remembers the choice). The colours live in the
// stylesheet; this file only decides the order and the chip previews.

export interface Theme {
  id: string;
  scheme: 'dark' | 'light';
  /** [bg, surface, fg, dim, accent] — exactly what a chip shows. */
  colors: readonly [string, string, string, string, string];
}

export const THEMES: readonly Theme[] = [
  { id: 'wr', scheme: 'dark', colors: ['#07090d', '#0d1117', '#c9d3e3', '#5f7089', '#4da3ff'] },
  { id: 'gruvbox', scheme: 'dark', colors: ['#282828', '#32302f', '#ebdbb2', '#928374', '#fabd2f'] },
  { id: 'nord', scheme: 'dark', colors: ['#2e3440', '#343b49', '#d8dee9', '#616e88', '#88c0d0'] },
  { id: 'catppuccin', scheme: 'dark', colors: ['#1e1e2e', '#181825', '#cdd6f4', '#6c7086', '#cba6f7'] },
  { id: 'solarized', scheme: 'dark', colors: ['#002b36', '#073642', '#93a1a1', '#586e75', '#268bd2'] },
  { id: 'matrix', scheme: 'dark', colors: ['#050805', '#0a120a', '#b6f3c4', '#4e8a5b', '#2ee66f'] },
  { id: 'paper', scheme: 'light', colors: ['#f2efe9', '#e9e5db', '#20242a', '#6b7178', '#0b5cad'] },
];

export const DEFAULT_THEME = 'wr';
export const LIGHT_THEME = 'paper';
export const STORAGE_KEY = 'wr-docs-theme';

export function themeById(id: string): Theme {
  return THEMES.find((t) => t.id === id) ?? THEMES[0]!;
}
