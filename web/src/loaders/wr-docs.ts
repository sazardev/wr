import { z } from 'astro:content';
import type { Loader, LoaderContext } from 'astro/loaders';
import { parseChangelog } from '../lib/changelog';
import { repoFile } from '../lib/paths';
import { readRepo } from '../lib/repo';
import { rewriteDocLinks } from '../lib/rewrite';
import { leadParagraph, splitDocument } from '../lib/split';
import { DOCS } from '../lib/registry';

// A custom loader for the Content Layer: it reads the documents of the
// repository (docs/GUIDE.md, docs/DESIGN.md, CHANGELOG.md) and stores **one
// entry per section**, so every `##` heading of the guide is a page of its own
// (/docs/guide/start/, /docs/guide/following-links/, …) with its own headings,
// summary and search text.
//
// Adding a document to the site means adding a line to DOCS — nothing else.

interface SectionSource {
  /** Route segment: a slug, 'index' for the intro, 'v0.3.0' for a release. */
  section: string;
  order: number;
  title: string;
  summary: string;
  date?: string;
  editAnchor?: string;
  markdown: string;
  text: string;
}

export const docsSchema = z.object({
  doc: z.string(),
  docLabel: z.string(),
  docFile: z.string(),
  glyph: z.string(),
  /** 'index' for the intro page, otherwise the section slug. */
  section: z.string(),
  order: z.number(),
  title: z.string(),
  summary: z.string(),
  /** ISO date, only the changelog carries one. */
  date: z.string().optional(),
  /** Anchor in the source file, for the "edit this page" link. */
  editAnchor: z.string().optional(),
  /** Headings inside the section, for its table of contents. */
  headings: z.array(z.object({ depth: z.number(), slug: z.string(), text: z.string() })),
  /** Plain text of the section, for the search index. */
  text: z.string(),
});

/** How a source file becomes sections: per `##` heading, or per release. */
function sectionsOf(docId: string, markdown: string): { intro: SectionSource; rest: SectionSource[] } {
  if (docId === 'changelog') {
    const changelog = parseChangelog(markdown);
    const intro: SectionSource = {
      section: 'index',
      order: 0,
      title: changelog.title,
      summary: 'Every release of wr, written by release-please.',
      markdown: changelog.intro,
      text: '',
    };
    const rest: SectionSource[] = changelog.releases.map((release, i) => ({
      section: `v${release.version}`,
      order: i + 1,
      title: `Release ${release.version}`,
      summary: release.date ?? '',
      date: release.date ?? undefined,
      markdown: release.markdown,
      text: release.text,
    }));
    return { intro, rest };
  }

  const doc = splitDocument(markdown);
  const intro: SectionSource = {
    section: 'index',
    order: 0,
    title: doc.title,
    summary: doc.summary,
    markdown: doc.introMarkdown,
    text: '',
  };
  const rest: SectionSource[] = doc.sections.map((section) => ({
    section: section.slug,
    order: section.order,
    title: section.heading,
    summary: leadParagraph(section.markdown),
    editAnchor: section.slug,
    markdown: section.markdown,
    text: '',
  }));
  return { intro, rest };
}

/** Very small plain-text conversion, enough for a search index. */
function plainText(markdown: string): string {
  return markdown
    .replace(/```[\s\S]*?```/g, (fence) => fence.replace(/^```\w*|```$/gm, ' '))
    .replace(/`([^`]*)`/g, '$1')
    .replace(/!\[[^\]]*]\([^)]*\)/g, ' ')
    .replace(/\[([^\]]*)]\([^)]*\)/g, '$1')
    .replace(/^\s{0,3}#{1,6}\s+/gm, '')
    .replace(/^\s{0,3}>+\s?/gm, '')
    .replace(/^\s*[-*+]\s+/gm, '')
    .replace(/^\s*\d+\.\s+/gm, '')
    .replace(/[*_~]/g, '')
    .replace(/\|/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
}

async function loadDoc(context: LoaderContext, meta: (typeof DOCS)[number]): Promise<void> {
  const { store, parseData, renderMarkdown, generateDigest, logger } = context;
  for (const key of store.keys()) {
    if (key.startsWith(`${meta.id}/`)) store.delete(key);
  }
  const source = readRepo(meta.file);
  const digest = generateDigest(source);
  const { intro, rest } = sectionsOf(meta.id, source);
  const slugs = new Set([...rest.map((s) => s.section), 'index']);

  for (const section of [intro, ...rest]) {
    const id = `${meta.id}/${section.section}`;
    const rendered = await renderMarkdown(section.markdown);
    const headings = (rendered.metadata?.headings ?? []).map((heading) => ({
      depth: heading.depth,
      slug: heading.slug,
      text: heading.text,
    }));
    const data = await parseData({
      id,
      data: {
        doc: meta.id,
        docLabel: meta.label,
        docFile: meta.file,
        glyph: meta.glyph,
        section: section.section,
        order: section.order,
        title: section.title,
        summary: section.summary,
        date: section.date,
        editAnchor: section.editAnchor,
        headings,
        text: section.text || plainText(section.markdown),
      },
    });
    store.set({
      id,
      data,
      digest,
      rendered: {
        html: rewriteDocLinks(rendered.html, meta.id, slugs),
        metadata: rendered.metadata,
      },
    });
    logger.debug(`wr-docs: ${id} — ${section.title}`);
  }
}

export function wrDocsLoader(): Loader {
  return {
    name: 'wr-docs-loader',
    async load(context) {
      context.store.clear();
      for (const meta of DOCS) {
        await loadDoc(context, meta);
      }
      // In dev, editing a source document rebuilds its pages on the spot.
      context.watcher?.on('change', (file) => {
        const meta = DOCS.find((doc) => repoFile(doc.file) === file);
        if (meta) void loadDoc(context, meta);
      });
    },
    schema: docsSchema,
  };
}
