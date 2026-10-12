import { go } from './go';
import { closeHelp, helpOpen, openHelp } from './help';
import { toggle, open as openPalette } from './palette';
import { openThemeMenu } from './themes';
import { withBase } from '../lib/urls';

// The site behaves like the program it documents: the keyboard gets you
// everywhere. Ctrl+K and "/" search, "[" and "]" turn the pages of a document,
// "t" changes the theme, "g h" / "g d" jump home / to the docs, "?" lists them.
// A dialog owns the keyboard while it is open; typing in a field is never
// intercepted.

function isTyping(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  return el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement || el.isContentEditable;
}

function modalOpen(): boolean {
  return document.querySelector('[data-modal]:not([hidden])') !== null;
}

function turn(rel: 'prev' | 'next'): void {
  const link = document.querySelector<HTMLAnchorElement>(`.pager a[rel="${rel}"]`);
  if (link) go(link.href);
}

let chord = false;
let chordTimer: ReturnType<typeof setTimeout> | undefined;
let wired = false;

export function startHotkeys(): void {
  if (wired) return;
  wired = true;

  document.addEventListener('keydown', (event) => {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      if (helpOpen()) closeHelp();
      toggle();
      return;
    }
    if (event.metaKey || event.ctrlKey || event.altKey) return;

    if (modalOpen()) {
      if (helpOpen() && (event.key === 'Escape' || event.key === '?')) {
        event.preventDefault();
        closeHelp();
      }
      return;
    }
    if (isTyping(event.target)) {
      if (event.key === 'Escape') (event.target as HTMLElement).blur();
      return;
    }

    if (chord) {
      chord = false;
      clearTimeout(chordTimer);
      if (event.key === 'h') go(withBase('/'));
      else if (event.key === 'd') go(withBase('/docs/'));
      else return;
      event.preventDefault();
      return;
    }

    switch (event.key) {
      case '/': {
        event.preventDefault();
        const field = document.querySelector<HTMLInputElement>('[data-search-input]');
        if (field) {
          field.focus();
          field.select();
        } else openPalette();
        break;
      }
      case '?':
        event.preventDefault();
        openHelp();
        break;
      case 't':
        event.preventDefault();
        openThemeMenu();
        break;
      case '[':
        turn('prev');
        break;
      case ']':
        turn('next');
        break;
      case 'g':
        chord = true;
        chordTimer = setTimeout(() => (chord = false), 900);
        break;
    }
  });
}
