import { defineCollection } from 'astro:content';
import { wrDocsLoader } from './loaders/wr-docs';

// The 'docs' collection is not made of files inside src/content: it is built
// by the loader from the repository's own Markdown, split into one entry per
// section. See src/loaders/wr-docs.ts.
const docs = defineCollection({
  loader: wrDocsLoader(),
});

export const collections = { docs };
