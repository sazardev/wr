// @ts-check
import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';
import rehypeSlug from 'rehype-slug';
import rehypeRaw from 'rehype-raw';
import { rehypeMedia } from './src/lib/rehype-media.mjs';

// The site is published on Cloudflare at the root of wr.sazardev.stream.
// Both the origin and the base path are environment-driven so a preview or a
// fork can build the same pages under another host or path.
const site = process.env.SITE_URL ?? 'https://wr.sazardev.stream';
const base = process.env.SITE_BASE ?? '/';

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
    // each token and the stylesheet (styles/tui.css) paints them with the
    // active palette, so code follows every theme with no per-theme overrides.
    gfm: true,
    smartypants: true,
    rehypePlugins: [rehypeSlug, rehypeRaw, rehypeMedia],
    shikiConfig: {
      theme: 'css-variables',
    },
  },
});
