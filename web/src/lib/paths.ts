import { existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// This file lives at <repo>/web/src/lib, or, once bundled, under
// <repo>/web/dist/.prerender, so the site root is found by walking up to its
// Astro config instead of counting directories. Everything the docs are generated
// from lives inside the repository, so the site never needs a network fetch
// at build time and can never drift from the source of truth.
const here = path.dirname(fileURLToPath(import.meta.url));

function findWebRoot(from: string): string {
  for (let dir = from; ; dir = path.dirname(dir)) {
    if (existsSync(path.join(dir, 'astro.config.mjs'))) return dir;
    if (dir === path.dirname(dir)) throw new Error(`no astro.config.mjs above ${from}`);
  }
}

/** Root of the documentation site (…/web). */
export const webRoot = findWebRoot(here);
/** Root of the wr repository, the parent of the site. */
export const repoRoot = path.dirname(webRoot);

/** Absolute path of a file inside the repository. */
export function repoFile(...segments: string[]): string {
  return path.join(repoRoot, ...segments);
}

/** Absolute path of a file inside the documentation site. */
export function webFile(...segments: string[]): string {
  return path.join(webRoot, ...segments);
}
