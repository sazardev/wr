import { readFileSync } from 'node:fs';
import { repoFile } from './paths';

// Build-time access to the repository's source of truth. Reading is memoized:
// the loader, the reference pages and the search index all ask for the same
// files, and none of them may drift.

const cache = new Map<string, string>();

/** Contents of a file in the repository, relative to its root. */
export function readRepo(relative: string): string {
  const cached = cache.get(relative);
  if (cached !== undefined) return cached;
  const source = readFileSync(repoFile(relative), 'utf8');
  cache.set(relative, source);
  return source;
}

export const readGuide = () => readRepo('docs/GUIDE.md');
export const readDesign = () => readRepo('docs/DESIGN.md');
export const readChangelog = () => readRepo('CHANGELOG.md');
export const readReadme = () => readRepo('README.md');
export const readKeysGo = () => readRepo('keys.go');
export const readManifest = () => readRepo('.release-please-manifest.json');
