import { navigate } from 'astro:transitions/client';

/** Opens an address: through the view-transition router when it is a page of
 *  this site, with a plain navigation when it leaves it. */
export function go(href: string): void {
  const url = new URL(href, location.href);
  if (url.origin !== location.origin) {
    location.assign(url.href);
    return;
  }
  void navigate(url.pathname + url.search + url.hash);
}
