import { slug } from 'github-slugger';

// Splits a repository Markdown document into the pieces the site routes over:
// one page per `##` section, plus an index page for the title and intro.
//
// Everything here is pure string work on one purpose: keep the Markdown file
// the single source of truth (it ships inside the release tarballs), while the
// site gets deep routes, per-section navigation and search from the same text.

export interface DocumentSection {
  /** Position in the document, starting at 1. */
  order: number;
  /** GitHub-style anchor of the heading: /docs/guide/<slug>/. */
  slug: string;
  /** Heading text with the Markdown syntax removed (`The screen`, not `` `The screen` ``). */
  heading: string;
  /** Markdown of the section body, without its `##` heading. */
  markdown: string;
}

export interface SplitDocument {
  /** The `# title` of the document. */
  title: string;
  /** First paragraph of the intro: used as the page description. */
  summary: string;
  /** Markdown of the intro, without the title and without its manual TOC. */
  introMarkdown: string;
  sections: DocumentSection[];
}

const FENCE = /^\s*(```|~~~)/;
const HEADING2 = /^##\s+(.+?)\s*$/;
const TITLE1 = /^#\s+(.+?)\s*$/;
const TOC_ITEM = /^\s*[-*]\s+\[[^\]]*]\(#[^)]*\)\s*$/;

/** Removes Markdown inline syntax so a heading reads as plain text. */
export function plainText(source: string): string {
  return source
    .replace(/`([^`]*)`/g, '$1')
    .replace(/\*\*([^*]*)\*\*/g, '$1')
    .replace(/\*([^*]*)\*/g, '$1')
    .replace(/\[([^\]]*)]\([^)]*\)/g, '$1')
    .trim();
}

/** The first paragraph that is not the manual table of contents. */
function firstParagraph(lines: string[]): string {
  for (const line of lines) {
    if (!line.trim() || TOC_ITEM.test(line) || /^[-*]\s/.test(line.trim())) continue;
    return plainText(line);
  }
  return '';
}

/**
 * Drops a list of anchor links: the site builds its own table of contents, and
 * keeping the handwritten one would show the reader two of them. A run of two
 * or more anchor-only list items counts as a table of contents; a lone link is
 * left alone.
 */
function withoutTocList(lines: string[]): string[] {
  const out: string[] = [];
  let run: string[] = [];
  const flush = () => {
    if (run.length < 2) out.push(...run);
    run = [];
  };
  for (const line of lines) {
    if (TOC_ITEM.test(line)) run.push(line);
    else {
      flush();
      out.push(line);
    }
  }
  flush();
  return out
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
    .split('\n');
}

/** Splits a document at its `##` headings, ignoring fenced code blocks. */
export function splitDocument(markdown: string): SplitDocument {
  const source = stripFrontmatter(markdown);
  const lines = source.split('\n');

  const title = lines.map((l) => l.match(TITLE1)).find((m) => m)?.[1] ?? '';
  let intro: string[] = [];
  const sections: DocumentSection[] = [];

  let fenced = false;
  let heading: string | null = null;
  let body: string[] = [];

  const flush = () => {
    if (heading === null) {
      intro = body;
      return;
    }
    sections.push({
      order: sections.length + 1,
      slug: slug(plainText(heading)),
      heading: plainText(heading),
      markdown: body.join('\n').trim(),
    });
  };

  for (const line of lines) {
    if (FENCE.test(line)) fenced = !fenced;
    const match = !fenced ? line.match(HEADING2) : null;
    if (match) {
      flush();
      heading = match[1]!;
      body = [];
      continue;
    }
    body.push(line);
  }
  flush();

  const introLines = withoutTocList(
    intro.filter((line, i) => !(i === 0 && TITLE1.test(line))),
  );
  const introMarkdown = introLines.join('\n').trim();

  return {
    title: plainText(title),
    summary: firstParagraph(introLines),
    introMarkdown,
    sections,
  };
}

/** The first paragraph of a Markdown block, as plain text, skipping code. */
export function leadParagraph(markdown: string, limit = 240): string {
  const lines = markdown.split('\n');
  let fenced = false;
  for (const line of lines) {
    if (/^\s*(```|~~~)/.test(line)) {
      fenced = !fenced;
      continue;
    }
    if (fenced) continue;
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('|')) continue; // a table is not a summary
    const text = plainText(trimmed.replace(/^[-*+]\s+/, '').replace(/^\d+\.\s+/, ''));
    if (text.length > 3) return text.length > limit ? `${text.slice(0, limit).trimEnd()}…` : text;
  }
  return '';
}

/** Removes a leading `---` YAML block, if the document has one. */
function stripFrontmatter(markdown: string): string {
  const match = markdown.match(/^---\r?\n[\s\S]*?\r?\n---\r?\n?/);
  return match ? markdown.slice(match[0].length) : markdown;
}
