import { readFileSync } from 'node:fs';
import { repoFile } from './paths';

// The version of the documentation is the version the repository is at: it is
// read from the release-please manifest, the same file release-please bumps on
// every release. WR_VERSION overrides it (used by the deploy workflow to stamp
// a tagged build).
let cached: string | undefined;

export function siteVersion(): string {
  if (cached !== undefined) return cached;
  const fromEnv = process.env.WR_VERSION?.trim();
  if (fromEnv) {
    cached = fromEnv.replace(/^v/, '');
    return cached;
  }
  try {
    const manifest = JSON.parse(readFileSync(repoFile('.release-please-manifest.json'), 'utf8')) as Record<
      string,
      string
    >;
    cached = manifest['.'] ?? 'dev';
  } catch {
    cached = 'dev';
  }
  return cached;
}
