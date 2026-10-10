import { withBase } from '../lib/urls';

// The site behaves like the program it documents: `/` starts a search from
// anywhere, `Esc` lets go of the input. On the search page it focuses the box;
// anywhere else it opens the search page.

function isTyping(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  return el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || el.isContentEditable;
}

export function startHotkeys(): void {
  if (document.body.dataset.hotkeysOn === '1') return;
  document.body.dataset.hotkeysOn = '1';

  document.addEventListener('keydown', (event) => {
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === 'Escape' && document.activeElement instanceof HTMLInputElement) {
      document.activeElement.blur();
      return;
    }
    if (event.key !== '/' || isTyping(event.target)) return;
    event.preventDefault();
    const input = document.querySelector<HTMLInputElement>('[data-search-input]');
    if (input) {
      input.focus();
      input.select();
      return;
    }
    location.assign(withBase('/docs/search/'));
  });
}
