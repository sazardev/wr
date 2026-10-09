# QA report: reading the web and rendering code

Status: findings only. No code was changed for this report; every item below has
a minimal reproduction. This is the record of two testing rounds against the
binary built from commit `fcb54cf` (`wr v0.2.1-0.20261009005749-fcb54cf0cd4f`).

- [When](#when)
- [Where it was tested](#where-it-was-tested)
- [How it was tested](#how-it-was-tested)
- [Results](#results)
- [Why it fails: root causes](#why-it-fails-root-causes)
- [What already works](#what-already-works)
- [Minimal reproductions](#minimal-reproductions)
- [Known limits, not bugs](#known-limits-not-bugs)

## When

- Round 1 (web reading): 2026-10-08, 19:58–20:08 (America/Los_Angeles).
- Round 2 (code and languages): 2026-10-08, 20:44–20:55.
- Environment: Linux (CachyOS), Go 1.27.1, `wr` built from `fcb54cf` with
  `go build -ldflags="-s -w"`. Tests ran with an isolated `HOME` so the config,
  cache and history of the developer were never touched, and the cache is never
  read in pipe mode (`wr --md URL` and `wr doc.md | cat` always fetch fresh).

### Scope

| Round | Input | Count |
|---|---|---|
| 1 | URLs in 10 categories (blogs, docs, Wikipedia, news EN/ES, papers, gov, forums, code hosts, odd formats, JS/paywall sites) | 263 |
| 1b | Real articles harvested from those front pages | 113 |
| 2 | Language entries: 297 chroma lexers + 11 `wr` aliases, 262 with a real snippet (143 official chroma samples + 119 written for the test) | 308 |
| 2b | Edge cases (tabs, nested fences, real ESC bytes, long lines, CJK/RTL, CRLF, entities) | 12 |
| 2c | Language/framework documentation sites (zig, nim, numpy, pandas, Django, Rails, Godot, Vue, …) | 86 |

Total: 462 URLs, 179+ distinct domains, 320 code blocks, plus end-to-end copy
tests in a pseudo-terminal.

## Where it was tested

Categories: technical blogs, official docs, Wikipedia (EN/ES), news (EN/ES),
scientific papers/abstracts, government sites, forums and Q&A, code hosting
(READMEs, issues, raw files), odd content types (RFC text, JSON, XML feeds,
Markdown, CSV, PDF), JS-heavy sites and paywalls.

Failures cluster at these real pages (examples):

- `fasterthanli.me` articles — content destroyed by a `has-toc` class
  (`/articles/working-with-strings-in-rust`, `/articles/declarative-memory-management`).
- `gov.uk/government/news` and `/publications` — everything inside a `<form>`.
- Wikipedia — page title removed; Markdown and Python articles also lost the
  fallback title.
- Front pages / listings: `phoronix.com`, `seangoedecke.com`,
  `codeberg.org/forgejo/forgejo`, `fly.io/blog`, `charm.sh/blog`, `elmundo.es` —
  only the first `<article>` teaser survives.
- `w3.org/TR/html52/` — redirects to the WHATWG multipage index; only the stub
  remains after the TOC is dropped.
- Paywall/JS sites (external): `heise.de`, `lanacion.com.ar`, `xeiaso.net`
  (Cloudflare challenge), `openlibrary.org` (verification), `go.dev/tour`,
  `notion.so`, `x.com`, `instagram.com`.
- Code language detection: `ziglang.org/documentation/master` (831 untagged
  blocks), `nim-lang.org/docs/manual.html` (442), GNU `make` manual (397),
  `perldoc.perl.org/perlfunc` (354), `pkg.go.dev` (278), bash manual (178),
  Rails guides (150), numpy (138, `highlight-ipython`), Godot (123,
  `highlight-gdscript`), Django (120, `highlight-sql`), pandas (80).

## How it was tested

For every URL, three captures were stored side by side: the original response
(`curl`, wr's User-Agent, 25 s timeout), `wr --md URL` (extraction) and
`wr document.md | cat` (the non-interactive render). Then metrics were computed:

- coverage: fraction of sentences sampled from the extractable root
  (`<article>`, else `<main>`, else `<body>`, after scripts/nav removals) that
  appear in the produced Markdown;
- boilerplate leaks: cookies, newsletters, login, ads, share, related,
  comments, legal, social, nav, app-push and paywall patterns counted in the
  Markdown;
- structure: `<h1>`…`<h6>`, `<pre>`, `<table>`, `<img>`, `<a>` in the source vs
  headings, fences, table rows, `[image: …]` and links in the Markdown/rendered
  text;
- render: leftover Markdown syntax in the rendered output, ANSI color runs, and
  whether the rendered code body is byte-identical to the source;
- code: per language, whether the fence keeps the language, how many distinct
  ANSI colors the body gets, and whether indentation/blank lines survive;
- copy: `wr` was run inside a PTY, real SGR mouse clicks and drags were sent,
  and the OSC 52 clipboard payload was decoded and compared.

Complements: a Go probe replicating `toMarkdown` with per-rule word accounting
(dropTags, junkClass, codeJunk), synthetic HTML fixtures, and the official
chroma lexer list and samples (`lexers/testdata`).

## Results

### Web reading

| | Front pages/docs (263) | Real articles (113) |
|---|---|---|
| `wr` extracted something | 232 | 109 |
| near-empty (<50 words) | 30 | 9 |
| content captured ≥60% | 33% | **64%** |
| captured 30–60% | 33% | 21% |
| captured <30% | 35% | **15%** |
| median coverage | 0.40 | 0.73 |
| boilerplate leaks (median / worst) | 1 / 136 | 1 / 80 |
| `wr --md` time | 0.51 s median, 0.92 s p90 | — |
| render time | 19 ms median, 107 ms p90 | — |

Errors (34 total): 20 sites answered 401/403 to both `curl` and `wr` (paywalls
and bot walls: NYT, WSJ, FT, Economist, El País, Reuters, AP, Stack Exchange,
state.gov, NIH, science.org), 5 dead URLs, 4 TLS/network/protocol errors
(barrapunto certificate, washingtonpost HTTP/2 stream, semanticscholar and
amazon.es 202+EOF), 2 unsupported formats by design (PDF, PNG — with a clear
message), and 5 cases where `curl` succeeded but Go did not (france24 403,
ftc.gov 404, washingtonpost, semanticscholar, amazon.es).

Footnote lists (the default `links = "footnotes"`) are 13% of all rendered
lines globally; 40–47% on Wikipedia articles, 58% on the HN front page.

### Code and languages

| Metric | Result |
|---|---|
| blocks rendered with frame + label | 320/320 |
| entries with chromatic color | 304/308 (99%) |
| Markdown code body byte-identical to source | 272/308 (88%) |
| rendered code body byte-identical to source | 281/320 (88%) |
| fences WITH language on docs sites (rounds 1+2c) | 1,544/6,696 (**23%**) |
| fences WITH language on standalone articles | 80/121 (66%) |
| copy: drag block / double word / triple line | exact, verified via OSC 52 |

The 36 altered code samples break down as: 13 blank-line collapses, 19 trailing
spaces stripped, 3 charset mojibake, 1 other. All 297 lexers tokenize real code
and get color; the only colorless entries are `text`/`plaintext`/`txt` (by
design) and a sample that was a lone comment.

## Why it fails: root causes

1. **`junkClass` matches substrings inside class names** (`main.go:51`,
   removal at `main.go:336-340`). `(^|[\s_-])toc($|[\s_-])` matches
   `class="page-html has-toc"` and `Selection.Remove()` deletes the whole
   subtree: a fasterthanli.me article goes from 12,316 words to 0. It is a
   *contains* match, not an exact token.

2. **The extraction root is the *first* `<article>`** (`main.go:321-327`). On
   listings every teaser is an `<article>`; on some sites the first one is a
   banner or a single list item (phoronix: 8 of 4,929 words; codeberg: 5 of
   2,161; seangoedecke: 3). The root is chosen by order, not by text weight.

3. **`dropTags` removes whole `<form>` and `<header>` elements**
   (`main.go:46`, `main.go:333-335`). gov.uk wraps its news listing in
   `<form class="js-live-search-form">` → 5,040 words to 0. Wikipedia's `<h1>`
   lives inside `<header class="mw-body-header">` → the title disappears.

4. **The title fallback is skipped by a `#` anywhere in the text, fences
   included** (`main.go:389-391`). A code example whose first characters are
   `# ` (Wikipedia's Markdown/Python articles, shell comments in a ripgrep
   post) stops `<title>` from being prepended, so the page opens with no
   heading at all — on top of bug 3 for Wikipedia.

5. **The post-processing regexes run over the whole Markdown, fences
   included** (`main.go:382-384`): `\n{3,}` collapses consecutive blank lines
   inside code; `[ \t]+\n` deletes trailing spaces inside code; a code line
   made of 4+ Markdown links is deleted entirely. `sanitize()` (`main.go:395-399`)
   also strips real `ESC` bytes from code, so `\x1b[31m` becomes `[31m`.

6. **Charset detection falls back to windows-1252** (`main.go:270-276`). For
   HTML with no BOM and no `<meta charset>`, when the first non-ASCII byte
   appears after the detector's preview (~1 KB), UTF-8 is read as windows-1252:
   `»` becomes `Â»`, `español` becomes `espaÃ±ol`. Short files with accents near
   the top are decoded correctly, which makes the bug look random.

7. **Language detection only reads a few class conventions** (`main.go:47-52`,
   `main.go:281-312`). It understands `language-xx`, `highlight-source-xx`,
   `brush: xx` and `data-language`. It does not understand the two most common
   doc conventions: Sphinx/Pygments `highlight-python|ipython|default|sql|…`
   and MediaWiki `mw-highlight-lang-*`. When the site has no class at all
   (`<pre>`), the block is shown with the `text` label and no color — correct,
   but it is the majority case on documentation sites (77%).

8. **Tables are lost when a cell contains a list** (html-to-markdown v2 table
   plugin). Minimal repro: `<td><ul><li>a</li></ul></td>` flattens the whole
   table to plain lines. Wikipedia infoboxes are the most visible casualty:
   52% of pages that have `<table>` produce no table at all. `colspan` and
   `rowspan` are also flattened silently, and tables with no header row get a
   fake empty header.

9. **Listing noise and footnote explosion are not damped.** Sidebars that are
   not semantic (`role="navigation"` divs, "related" blocks) stay in; with
   `links = "footnotes"` every link becomes a numbered note, which dominates
   long articles (Wikipedia's Markdown page: 466 links, 2× the visible text).

## What already works

- The renderer itself: zero leftover Markdown syntax in 232/235 pages; tables,
  lists, quotes, task lists, strikethrough and autolinks render correctly;
  UTF-8/CJK/RTL/emoji/wide characters are safe.
- Code rendering when the language is known: labeled frame, per-language ANSI
  colors, exact indentation, 4-space tab expansion, `┆` continuation lines for
  long lines, internal fences escaped with longer fences, entities decoded.
- Copy: drag/double/triple click verified end-to-end (OSC 52 decoded): the code
  frame and gutter are excluded, indentation is kept, and lines wrapped on
  screen come back as one line.
- Failures are safe: a failed load never replaces the page being read, PDFs and
  images are refused with a helpful message, and control characters are
  stripped from untrusted content.
- Speed: `--md` median 0.51 s (almost all network), render median 19 ms, 320
  code blocks rendered without a single failure.

## Minimal reproductions

Content loss by class token (bug 1):

```html
<article>
  <div class="page-html has-toc">
    <h1>Title</h1><p>The entire article disappears because of "has-toc".</p>
  </div>
</article>
```

```sh
wr --md has-toc.html   # prints only "# Title"
```

Blank lines and trailing spaces inside code (bug 5):

```html
<article><pre class="language-python"><code>a = 1


b = 2   </code></pre></article>
```

```sh
wr --md code.html | cat -A   # the two blank lines become one, spaces are gone
```

A code line that looks like links disappears (bug 5):

```html
<article><pre class="language-text"><code>[a](u1) [b](u2) [c](u3) [d](u4)
keep me</code></pre></article>
```

Charset (bug 6): a UTF-8 file without `<meta charset>` whose first accented
character sits after ~1 KB decodes as windows-1252; moving the accent into the
first bytes fixes it.

Tables with a list in a cell (bug 8):

```html
<table><tr><th>K</th><td><ul><li>a</li><li>b</li></ul></td></tr></table>
```

```sh
wr --md table.html   # "K" and the list, no table
```

## Known limits, not bugs

- JavaScript-rendered pages and paywalls: documented in the README; `wr` shows
  whatever the server returns and cannot flag it as a wall.
- Sites that answer 401/403 to any non-browser client (Stack Exchange, NYT,
  WSJ, …) are unreachable regardless of the extractor.
- Search is per rendered line; in-page `#links` match heading text only;
  languages without a chroma lexer (e.g. `mermaid`, RFC-specific classes) are
  shown uncolored — all documented in **Limits**.
- Coverage numbers use sentence sampling of the source root, so boilerplate
  that is *correctly* dropped lowers the score; they are a comparative signal,
  not an absolute quality rating.
