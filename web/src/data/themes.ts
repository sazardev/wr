// Theme data, shared by the server (which renders the chips) and the client
// script (which applies and remembers the choice). The colours live in
// styles/tokens.css; this file only decides the order and the chip previews.
//
// Every palette is monochrome: one hue, a ramp of it for the surfaces and the
// lines, and the same hue at full strength as the accent.

export interface Theme {
  id: string;
  scheme: 'dark' | 'light';
  /** [bg, panel, fg, dim, accent] — exactly what a chip shows. */
  colors: readonly [string, string, string, string, string];
}

export const THEMES: readonly Theme[] = [
  { id: 'blueprint', scheme: 'dark', colors: ['#050912', '#0a1222', '#e6eefc', '#647a9e', '#6cb4ff'] },
  { id: 'graphite', scheme: 'dark', colors: ['#0a0a0a', '#121212', '#ededed', '#7a7a7a', '#ffffff'] },
  { id: 'phosphor', scheme: 'dark', colors: ['#040a06', '#08140c', '#d2f5dc', '#4f8a62', '#3dff85'] },
  { id: 'amber', scheme: 'dark', colors: ['#0b0802', '#150f04', '#f7e6c4', '#8f7440', '#ffb224'] },
  { id: 'paper', scheme: 'light', colors: ['#f4f7fc', '#e9eff8', '#0b1f44', '#5c6f93', '#1550c8'] },
];

export const DEFAULT_THEME = 'blueprint';
export const LIGHT_THEME = 'paper';
export const STORAGE_KEY = 'wr-docs-theme';

export function themeById(id: string): Theme {
  return THEMES.find((t) => t.id === id) ?? THEMES[0]!;
}
