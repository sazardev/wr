// The two drawers of the narrow layout: the site menu under the header and the
// document's own table of contents under the doc bar.

function setMenu(open: boolean): void {
  const menu = document.querySelector<HTMLElement>('[data-mobile-nav]');
  const toggle = document.querySelector<HTMLElement>('[data-nav-toggle]');
  if (!menu || !toggle) return;
  menu.hidden = !open;
  toggle.setAttribute('aria-expanded', String(open));
  toggle.querySelector<HTMLElement>('[data-nav-icon="open"]')!.hidden = open;
  toggle.querySelector<HTMLElement>('[data-nav-icon="close"]')!.hidden = !open;
  document.body.classList.toggle('lock', open);
}

function setAside(open: boolean): void {
  const aside = document.querySelector<HTMLElement>('[data-aside]');
  const toggle = document.querySelector<HTMLElement>('[data-aside-toggle]');
  if (!aside || !toggle) return;
  aside.toggleAttribute('data-open', open);
  toggle.setAttribute('aria-expanded', String(open));
  document.body.classList.toggle('lock', open);
}

let wired = false;

export function startNav(): void {
  // a fresh document always starts with both drawers shut
  document.body.classList.remove('lock');
  if (wired) return;
  wired = true;
  document.addEventListener('click', (event) => {
    const target = event.target as HTMLElement;
    if (target.closest('[data-nav-toggle]')) {
      setMenu(document.querySelector<HTMLElement>('[data-mobile-nav]')?.hidden ?? false);
      return;
    }
    if (target.closest('[data-aside-toggle]')) {
      setAside(!document.querySelector('[data-aside]')?.hasAttribute('data-open'));
      return;
    }
    if (target.closest('[data-mobile-nav] a')) setMenu(false);
    if (target.closest('[data-aside] a')) setAside(false);
  });
  addEventListener('resize', () => {
    if (innerWidth > 900) setMenu(false);
    if (innerWidth > 820) setAside(false);
  });
}
