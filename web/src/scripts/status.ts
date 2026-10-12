import { withBase } from '../lib/urls';

// The reading position, in three places: the thin bar under the header, the
// percentage and the path in the status line at the bottom. Listeners are bound
// once; the elements are looked up on every paint, because a navigation
// replaces them.

const BASE = withBase('/').replace(/\/$/, '');
const MODES: Record<string, string> = {
  guide: 'GUIDE', design: 'DESIGN', changelog: 'LOG', reference: 'REF', search: 'FIND',
};

function route(): { path: string; mode: string } {
  const rest = location.pathname.startsWith(BASE) ? location.pathname.slice(BASE.length) : location.pathname;
  const parts = rest.split('/').filter(Boolean);
  const path = parts.length ? `~/${parts.join('/')}` : '~/';
  if (parts.length === 0) return { path, mode: 'WR' };
  if (parts[0] !== 'docs') return { path, mode: parts[0]!.toUpperCase().slice(0, 8) };
  return { path, mode: MODES[parts[1] ?? ''] ?? 'DOCS' };
}

export function paintStatus(): void {
  const doc = document.documentElement;
  const max = doc.scrollHeight - innerHeight;
  const p = max > 0 ? Math.min(1, Math.max(0, scrollY / max)) : 0;
  const bar = document.querySelector<HTMLElement>('[data-progress]');
  if (bar) bar.style.setProperty('--p', p.toFixed(4));
  const pct = document.querySelector<HTMLElement>('[data-st-pct]');
  if (pct) pct.textContent = p < 0.005 ? 'top' : p > 0.995 ? 'end' : `${Math.round(p * 100)}%`;
}

let wired = false;

export function startStatus(): void {
  const here = route();
  const path = document.querySelector<HTMLElement>('[data-st-path]');
  const mode = document.querySelector<HTMLElement>('[data-st-mode]');
  if (path) path.textContent = here.path;
  if (mode) mode.textContent = here.mode;
  paintStatus();
  if (wired) return;
  wired = true;
  let queued = false;
  const schedule = (): void => {
    if (queued) return;
    queued = true;
    requestAnimationFrame(() => {
      queued = false;
      paintStatus();
    });
  };
  addEventListener('scroll', schedule, { passive: true });
  addEventListener('resize', schedule);
}
