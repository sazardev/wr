// "On this page": the heading being read is marked in the table of contents,
// and the marked link scrolls itself into view in a long list.

let wired = false;

function update(): void {
  const links = [...document.querySelectorAll<HTMLAnchorElement>('[data-toc] a[href^="#"]')];
  if (!links.length) return;
  const line = 120;
  let active: HTMLAnchorElement | undefined;
  for (const link of links) {
    const id = decodeURIComponent(link.hash.slice(1));
    const heading = document.getElementById(id);
    if (heading && heading.getBoundingClientRect().top <= line) active = link;
  }
  // at the very end of the page the last heading wins even if it never reaches the line
  if (innerHeight + scrollY >= document.documentElement.scrollHeight - 4) active = links[links.length - 1];
  for (const link of links) {
    const on = link === active;
    link.classList.toggle('is-active', on);
    if (on) link.setAttribute('aria-current', 'location');
    else link.removeAttribute('aria-current');
  }
}

export function startScrollspy(): void {
  update();
  if (wired) return;
  wired = true;
  let queued = false;
  addEventListener(
    'scroll',
    () => {
      if (queued) return;
      queued = true;
      requestAnimationFrame(() => {
        queued = false;
        update();
      });
    },
    { passive: true },
  );
}
