import path from 'node:path';
import { fileURLToPath } from 'node:url';

// This file lives at <repo>/web/src/lib. Everything the docs are generated
// from lives inside the repository, so the site never needs a network fetch
// at build time and can never drift from the source of truth.
const here = path.dirname(fileURLToPath(import.meta.url));

/** Root of the wr repository (…/web/src/lib/../../..). */
export const repoRoot = path.resolve(here, '../../..');
/** Root of the documentation site (…/web). */
export const webRoot = path.resolve(here, '../..');

/** Absolute path of a file inside the repository. */
export function repoFile(...segments: string[]): string {
  return path.join(repoRoot, ...segments);
}

/** Absolute path of a file inside the documentation site. */
export function webFile(...segments: string[]): string {
  return path.join(webRoot, ...segments);
}
