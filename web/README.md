# web/ — the wr documentation site

The reader's TUI, drawn as a static [Astro](https://astro.build/) site. It is
published on **Cloudflare Pages** at <https://wr.sazardev.stream>.

```sh
npm ci             # once; the lockfile is committed
npm run verify     # astro check + vitest + production build
npm run dev        # local preview at /
npm run deploy     # build + wrangler pages deploy
```

## Production

- **Cloudflare Pages** project: `wr`
- **Production URL**: <https://wr.sazardev.stream>
- **Pages URL**: <https://wr-9es.pages.dev>
- **Build**: `npm run build` → `dist/` (`wrangler.toml` points Pages at it)
- **Headers**: `public/_headers` (security headers and immutable `/_astro/*`)

The custom domain needs this record in the `sazardev.stream` zone:

| Type | Name | Content | Proxy |
|---|---|---|---|
| CNAME | `wr` | `wr-9es.pages.dev` | Proxied |

Cloudflare Pages verifies the CNAME and issues the certificate automatically.

## GitHub Actions

`.github/workflows/docs.yml` verifies every PR. On `main` it publishes with
`wrangler pages deploy` when the repository secret `CLOUDFLARE_API_TOKEN`
(Pages: Edit) exists; without it the deploy step is skipped with a warning. The
account id is public and lives in the workflow.

## How it is put together

The Markdown never moves: `docs/GUIDE.md`, `docs/DESIGN.md`,
`docs/BENCHMARKS.md`, `docs/ARCHITECTURE.md` and `CHANGELOG.md` stay in the
repository (the guide also ships inside the release tarballs). The site is
*generated* from them.

```
docs/*.md ─┐
CHANGELOG.md ─┼─▶ src/loaders/wr-docs.ts ─▶ content collection "docs"
              │        (one entry per ## section)      │
              │                                        ├─▶ /docs/guide/<section>/
keys.go ──────┐                                        ├─▶ /docs/design/<section>/
README.md ────┼─▶ src/lib/reference.ts ─▶ /docs/reference/…  └─▶ /docs/changelog/vX.Y.Z/
              │
              └─▶ src/pages/docs/search-index.json.ts ─▶ the search index
```

- **`src/loaders/wr-docs.ts`** — a content-layer loader. It reads the
  registered documents (`src/lib/registry.ts`), splits them at their `##`
  headings (`src/lib/split.ts`) and stores one entry per section, rendered with
  Astro's own Markdown pipeline (GFM, smart quotes, Shiki, heading ids).
- **`src/pages/docs/[doc]/[section].astro`** — the deep route. `getStaticPaths`
  walks the collection, so adding a section to `docs/GUIDE.md` adds a page.
- **`src/lib/reference.ts`** — generates the reference pages from the program's
  own sources: shortcuts from `keys.go`, settings and paths from the README.
- **`src/lib/search.ts`** — a small, tested scorer used by the address bar and
  the search page. The index is a JSON file built at build time.
- **`src/layouts/Base.astro`** — the terminal frame: header, braille rules,
  scrollable body, scrollbar and footer. The menu, help and settings panels are
  server-rendered; the content is the slot.
- **`src/styles/tui.css`** — the whole visual language: the 16 ANSI colors, the
  14 accents, the reader's glyphs, the prose, the code frames and the panels.
- **`src/scripts/tui.ts`** — the interaction: scroll, progress, search, panels,
  link labels, address bar, history, settings and the reading position.
- **`src/lib/tui.ts`** — the pure braille math (progress bar, scrollbar) with
  its unit test.
- **`src/lib/rehype-media.mjs`** — turns Markdown images and `<video>` blocks
  into inert placeholders. Nothing is downloaded until the reader turns on
  Images or Videos in Settings.
- **`src/scripts/tui.ts`** — also owns the media layer: lazy hydration with
  `IntersectionObserver`, the TUI frame, image/poster Cache Storage with its
  size in Settings, and the clear action.
- **`public/media/`** — the demo image/video used by the Design → Media page.
- **`docs/DESIGN.md`** — *The web edition: wr's TUI in a browser*.

## Conventions

- Every module in `src/lib/` is pure and has a unit test (`npm test`); Astro
  lives in the pages, the layouts and the content loader.
- Links always go through `src/lib/urls.ts` (`withBase`, `docPath`), so the
  site works under any base path (`SITE_BASE`) and origin (`SITE_URL`).
- The version shown everywhere is the repository's:
  `.release-please-manifest.json`, overridable with `WR_VERSION`.
- Editing the documentation means editing `docs/*.md` — this app only decides
  how it is presented.
