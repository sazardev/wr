// @ts-check
import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';
import rehypeSlug from 'rehype-slug';

// The site is published by GitHub Pages as a project site (…/wr/). Both the
// origin and the base path are environment-driven so the same build can be
// served from a custom domain, a preview or a fork.
const site = process.env.SITE_URL ?? 'https://sazardev.github.io';
const base = process.env.SITE_BASE ?? '/wr/';

export default defineConfig({
  site,
  base,
  output: 'static',
  trailingSlash: 'always',
  build: {
    // One stylesheet inlined per page keeps the payload at a single file.
    inlineStylesheets: 'auto',
  },
  integrations: [sitemap()],
  prefetch: {
    prefetchAll: true,
  },
  markdown: {
    // The pipeline the whole site uses: GFM tables, smart quotes, an id on
    // every heading (for the tables of contents and the section links) and
    // Shiki in its css-variables mode: the highlighter only names the kind of
    // each token and the stylesheet (styles/tokens.css) paints them with the
    // active palette, so code follows every theme with no per-theme overrides.
    gfm: true,
    smartypants: true,
    rehypePlugins: [rehypeSlug],
    shikiConfig: {
      theme: 'css-variables',
    },
  },
});
