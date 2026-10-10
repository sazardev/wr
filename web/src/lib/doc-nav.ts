import { getCollection, type CollectionEntry } from 'astro:content';

// Navigation over the docs collection: the entries of one document in reading
// order, and the neighbours of a section (the pager at the bottom of every
// page). The collection is in memory, so this is plain array work.

export type DocEntry = CollectionEntry<'docs'>;

/** Every entry of a document, the intro first, in reading order. */
export async function docEntries(doc: string): Promise<DocEntry[]> {
  const entries = await getCollection('docs');
  return entries
    .filter((entry) => entry.data.doc === doc)
    .sort((a, b) => a.data.order - b.data.order || a.data.section.localeCompare(b.data.section));
}

/** One entry of a document. */
export async function docEntry(doc: string, section: string): Promise<DocEntry | undefined> {
  const entries = await docEntries(doc);
  return entries.find((entry) => entry.data.section === section);
}

export interface Neighbours {
  previous: DocEntry | undefined;
  next: DocEntry | undefined;
}

/** The sections before and after one, skipping the document intro. */
export function neighboursOf(entries: readonly DocEntry[], section: string): Neighbours {
  const sections = entries.filter((entry) => entry.data.section !== 'index');
  const at = sections.findIndex((entry) => entry.data.section === section);
  return {
    previous: at > 0 ? sections[at - 1] : entries.find((entry) => entry.data.section === 'index'),
    next: at >= 0 && at < sections.length - 1 ? sections[at + 1] : undefined,
  };
}
