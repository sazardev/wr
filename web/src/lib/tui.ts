// The braille UI math of the reader, ported for the web. Keeping it here (and
// not in the DOM script) means the progress bar and the scrollbar can be
// tested without a browser.

export const FILL = ['⣀', '⣄', '⣤', '⣦', '⣶', '⣷', '⣿'] as const;
export const TRACK = '⡇';
export const PLAIN_TRACK = '│';
export const PLAIN_THUMB = '┃';

/** Filled and empty halves of a horizontal braille progress bar. */
export function progressBar(cells: number, pct: number, braille = true): { full: string; empty: string } {
  const n = Math.max(0, Math.floor(cells));
  if (n === 0) return { full: '', empty: '' };
  const clamped = Math.min(Math.max(pct, 0), 100);
  if (!braille) {
    const filled = Math.min(n, Math.max(0, Math.round((n * clamped) / 100)));
    return { full: '█'.repeat(filled), empty: '░'.repeat(n - filled) };
  }
  const steps = FILL.length - 1;
  const units = Math.round((clamped * n * steps) / 100);
  let full = '';
  for (let i = 0; i < n; i++) {
    const u = units - i * steps;
    if (u >= steps) full += FILL[steps];
    else if (u > 0) full += FILL[u];
    else break;
  }
  return { full, empty: FILL[0].repeat(n - full.length) };
}

const BRAILLE_ROW = [0x01 | 0x08, 0x02 | 0x10, 0x04 | 0x20, 0x40 | 0x80];

/**
 * One character per visible row, like the reader's scrollbar: a braille track
 * with a smooth thumb, or a plain `┃`/`│` bar when braille is off.
 * `top` is the first visible line (fractional while scrolling).
 */
export function scrollbarChars(
  h: number,
  total: number,
  view: number,
  top: number,
  braille = true,
): string[] {
  if (h <= 0 || total <= view) return [];
  if (!braille) {
    const thumb = Math.max(1, Math.floor((h * view) / total));
    const maxTop = total - view;
    const start = maxTop > 0 ? Math.round(((h - thumb) * top) / maxTop) : 0;
    const out: string[] = [];
    for (let i = 0; i < h; i++) {
      out.push(i >= start && i < start + thumb ? PLAIN_THUMB : PLAIN_TRACK);
    }
    return out;
  }
  const sub = h * 4;
  const thumb = Math.max(4, Math.floor((sub * view) / total));
  const maxTop = total - view;
  let start = maxTop > 0 ? Math.round(((sub - thumb) * top) / maxTop) : 0;
  start = Math.min(Math.max(start, 0), sub - thumb);
  const end = start + thumb;
  const out: string[] = [];
  for (let r = 0; r < h; r++) {
    let bits = 0;
    for (let k = 0; k < 4; k++) {
      const s = r * 4 + k;
      if (s >= start && s < end) bits |= BRAILLE_ROW[k]!;
    }
    out.push(bits === 0 ? TRACK : String.fromCharCode(0x2800 + bits));
  }
  return out;
}

/** Reading percentage, rounded like the reader's footer. */
export function readPercent(scrollTop: number, maxTop: number): number {
  if (maxTop <= 0) return 100;
  return Math.round((100 * Math.min(Math.max(scrollTop, 0), maxTop)) / maxTop);
}

/** Number of progress cells the footer uses at a given width. */
export function progressCells(width: number): number {
  if (width >= 110) return 16;
  if (width >= 80) return 12;
  if (width >= 60) return 8;
  if (width >= 45) return 5;
  return 0;
}
