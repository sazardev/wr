import { docPath, repo } from './urls';

// One place that rewrites the links of the generated pages: anchors that point
// at another section of the same document become the deep route, and links to
// Markdown files of the repository (../README.md) become their GitHub view.
// Pure functions over HTML strings, so they are unit-testable.

/** Turns `#section` links into routes when they name another section. */
export function rewriteSectionAnchors(html: string, doc: string, sectionSlugs: ReadonlySet<string>): string {
  return html.replace(/href="#([^"]+)"/g, (match, anchor: string) =>
    sectionSlugs.has(anchor) ? `href="${docPath(doc, anchor)}"` : match,
  );
}

/** Turns links to repository Markdown into links to their GitHub view. */
export function rewriteRepoLinks(html: string): string {
  return html.replace(/href="((?:\.\.?\/)?[\w./-]*\.md)(#[^"]*)?"/g, (_match, file: string, anchor?: string) => {
    const path = file.replace(/^\.\.?\//, '');
    const hash = anchor?.replace(/^#/, '');
    return `href="${repo.file(path, hash)}"`;
  });
}

/** Both rewrites, in the order the link text was written. */
export function rewriteDocLinks(html: string, doc: string, sectionSlugs: ReadonlySet<string>): string {
  return rewriteRepoLinks(rewriteSectionAnchors(html, doc, sectionSlugs));
}
