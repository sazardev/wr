// Base-aware URL helpers. The site is published under a base path (GitHub
// Pages project site), so every link in the app goes through withBase().

const raw = import.meta.env.BASE_URL ?? '/';
const base = raw.endsWith('/') ? raw.slice(0, -1) : raw;

/** Prefixes a site-absolute path (/docs/guide) with the configured base. */
export function withBase(path: string): string {
  return `${base}/${path.replace(/^\/+/, '')}`;
}

/** URL of a file in public/ (leading slash expected, e.g. /assets/scenes/ui.svg). */
export function assetUrl(path: string): string {
  return withBase(path);
}

/** Route of a docs page: docPath('guide', 'start') -> /docs/guide/start/. */
export function docPath(doc: string, section = ''): string {
  return withBase(`/docs/${doc}/${section ? `${section}/` : ''}`);
}

// GitHub links. The repository URL is the single place to point at a fork.
const ORIGIN = 'https://github.com/sazardev/wr';

export const repo = {
  url: ORIGIN,
  releases: `${ORIGIN}/releases`,
  latest: `${ORIGIN}/releases/latest`,
  issues: `${ORIGIN}/issues`,
  installScript: `https://raw.githubusercontent.com/sazardev/wr/main/install.sh`,
  /** Link to a file on the default branch, optionally to an anchor inside it. */
  file(path: string, anchor?: string): string {
    return `${ORIGIN}/blob/main/${path}${anchor ? `#${anchor}` : ''}`;
  },
};
