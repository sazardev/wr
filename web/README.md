# web/ — the wr documentation site

An [Astro](https://astro.build/) app that publishes this repository's
documentation to GitHub Pages (<https://sazardev.github.io/wr/>). Deployed by
`.github/workflows/docs.yml`.

```sh
npm ci          # once; the lockfile is committed
npm run dev     # the site at /wr/ (the Pages base path) with a live reload
npm run verify  # astro check + the unit tests + a production build
npm run build && npm run preview
```

## How it is put together

The Markdown never moves: `docs/GUIDE.md`, `docs/DESIGN.md` and `CHANGELOG.md`
stay in the repository (the guide also ships inside the release tarballs).
The site is *generated* from them.

```
docs/GUIDE.md ─┐
docs/DESIGN.md ─┼─▶ src/loaders/wr-docs.ts ─▶ content collection "docs"
CHANGELOG.md ───┘        (one entry per ## section)      │
                                                          ├─▶ /docs/guide/<section>/
keys.go ────────┐                                         ├─▶ /docs/design/<section>/
README.md ──────┼─▶ src/lib/reference.ts ─▶ /docs/reference/…  └─▶ /docs/changelog/vX.Y.Z/
                │
                └─▶ src/pages/docs/search-index.json.ts ─▶ the search index
```

- **`src/loaders/wr-docs.ts`** — a custom content-layer loader. It reads the
  registered documents (`src/lib/registry.ts`), splits them at their `##`
  headings (`src/lib/split.ts`) and stores one entry per section, rendered with
  Astro's own Markdown pipeline (GFM, smart quotes, Shiki, heading ids).
  Every `##` heading of the guide is therefore a page of its own, with its own
  title, summary and search text.
- **`src/pages/docs/[doc]/[section].astro`** — the deep route. `getStaticPaths`
  walks the collection, so adding a section to `docs/GUIDE.md` adds a page.
- **`src/lib/reference.ts`** — generates the reference pages from the program's
  own sources: the shortcuts are parsed out of `keys.go`, the settings and the
  paths out of the README tables.
- **`src/lib/search.ts`** — a small, tested scorer. The search page is static;
  its state is the query string (`/docs/search/?q=links&scope=guide`), and the
  index is a JSON file built at build time.
- **`src/scripts/`** — the client side, one module per job (themes, braille
  field, cursor, rules, progress, reel, search, hotkeys), composed by
  `enhance.ts` on load and after view-transition navigations.
- **`src/styles/site.css`** — the whole design system (DS-1.2): tokens, the
  seven palettes, the landing, the docs shell, the prose and the tables.

## Conventions

- Every module in `src/lib/` is pure and has a unit test (`npm test`); Astro
  lives in the pages, the layouts and the components.
- Links always go through `src/lib/urls.ts` (`withBase`, `docPath`), so the
  site works under any base path (`SITE_BASE`) and origin (`SITE_URL`).
- The version shown everywhere is the repository's:
  `.release-please-manifest.json`, overridable with `WR_VERSION`.
- Editing the documentation means editing `docs/*.md` — this app only decides
  how it is presented.
