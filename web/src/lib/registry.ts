// Which repository documents are published, and how they are presented.
//
// The Markdown stays where it has always been (docs/GUIDE.md ships inside the
// release tarballs, CHANGELOG.md is written by release-please): this registry
// is the only thing that decides what becomes a page, so publishing a new
// document means adding one line here.

export interface DocMeta {
  /** Collection id and first path segment of the route: /docs/<id>/. */
  id: string;
  /** Name shown in navigation and cards. */
  label: string;
  /** Braille glyph used as the doc's mark. */
  glyph: string;
  /** One sentence shown on the docs home and in search scope pickers. */
  blurb: string;
  /** File in the repository the pages are generated from. */
  file: string;
  /** Section the doc opens at when no section is given. */
  firstSection: string;
  /** Order in navigation. */
  order: number;
}

export const DOCS: readonly DocMeta[] = [
  {
    id: 'guide',
    label: 'Guide',
    glyph: '⣿',
    blurb: 'Read and browse the web from your terminal: every screen, every key, with examples.',
    file: 'docs/GUIDE.md',
    firstSection: 'start',
    order: 1,
  },
  {
    id: 'design',
    label: 'Design',
    glyph: '⡇',
    blurb: 'The design language of the program and this site: flat, Braille, sixteen colors.',
    file: 'docs/DESIGN.md',
    firstSection: 'rules',
    order: 2,
  },
  {
    id: 'changelog',
    label: 'Changelog',
    glyph: '⣄',
    blurb: 'Every release, written by release-please from the repository history.',
    file: 'CHANGELOG.md',
    firstSection: 'latest',
    order: 3,
  },
];

export function docMeta(id: string): DocMeta | undefined {
  return DOCS.find((d) => d.id === id);
}
