# QA findings register

Versioned record of every finding from the QA rounds documented in
[`QA.md`](QA.md) (round 1) and [`QA-v2.md`](QA-v2.md) (round 2). This file is the
source of truth for **what is open**; [`QA-PLAN.md`](QA-PLAN.md) is the source of
truth for **what to do and in what order**, and `qa_scenarios_test.go` is the
automatic test mode that covers each finding.

## How to read this register

| Field | Meaning |
|---|---|
| ID | `QA-F-NNN`, stable. Never reuse a number, even after a fix. |
| State | `Abierto` (open) · `En curso` · `Resuelto` · `Caracterización` (intended behavior, asserted as a regression test) · `Límite` (external to `wr`) |
| Severity | `Bloqueante` (page lost) · `Alta` (content wrong or unusable) · `Media` (noise/loss) · `Baja` (cosmetic) |
| Detectado en | Version/commit where the round ran. |
| Resuelto en | Version/commit that fixed it — fill in when it happens; the fix commit also flips the state. |
| Test | Subtest of `TestQA*` in `qa_scenarios_test.go` that asserts the fixed behavior. Open findings skip unless `WR_QA_STRICT=1`. |

A slipping doc/code drift is caught by `TestQAFindingsCovered`: every ID here must
have an entry in the coverage map of the test file, and vice versa.

---

## v0.2.1 — round 2026-10-08 (build `fcb54cf`)

463 URLs in 3 phases (front pages, harvested articles, language/framework docs),
308 language entries, 12 code edge cases, 20 synthetic fixtures and 4 clipboard
scenarios. Headline numbers: front pages captured ≥60% of content on 33% of
cases (64% on real articles); code blocks on documentation sites carried a
language only 23% of the time (66% on standalone articles); 88% byte fidelity for
rendered code.

| ID | State | Severity | Area | Summary |
|---|---|---|---|---|
| QA-F-001 | Resuelto | Bloqueante | Extraction | `junkClass` matches *substrings*: `class="… has-toc"` deletes the whole page |
| QA-F-002 | Resuelto | Alta | Extraction | Root = first `<article>`: only the first teaser survives listings |
| QA-F-003 | Resuelto | Alta | Extraction | `dropTags` deletes whole `<form>` elements (gov.uk news listing → 0 words) |
| QA-F-004 | Resuelto | Media | Extraction | `<header>` removed when the root is not `<article>`: Wikipedia's title disappears |
| QA-F-005 | Resuelto | Alta | Extraction | `<title>` fallback blocked by any `# ` line, including inside code fences |
| QA-F-006 | Resuelto | Alta | Code | Global regexes rewrite code inside fences: blank lines collapse, trailing spaces and link-like lines are deleted |
| QA-F-007 | Resuelto | Media | Code | `sanitize()` strips real `ESC` bytes from code blocks too |
| QA-F-008 | Resuelto | Alta | Code | Language detection knows 4 conventions; Sphinx `highlight-*`, MediaWiki `mw-highlight-lang-*` and plain `<pre>` are missed |
| QA-F-009 | Resuelto | Media | Code | A list (or colspan) inside a table cell destroys the table (html-to-markdown) |
| QA-F-010 | Resuelto | Alta | Charset | UTF-8 without a declared charset decodes as windows-1252 once the first non-ASCII byte is past the sniff window (~1 KB) → mojibake |
| QA-F-011 | Caracterización | Baja | Code | Tabs are expanded to 4 spaces for display and copy (asserted, documented) |
| QA-F-012 | Abierto | Media | UX | `links = "footnotes"` dominates long pages (13% of all rendered lines; 40–47% on Wikipedia) |
| QA-F-013 | Resuelto | Media | Extraction | Non-semantic wrappers (`role=navigation`, "related" blocks, cookie banners) are not dropped |

### QA-F-001 · `junkClass` matches substrings of class names

- **State**: Resuelto · **Severity**: Bloqueante · **Detectado en**: v0.2.1 (`fcb54cf`) · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: fasterthanli.me `/articles/working-with-strings-in-rust`,
  `/articles/declarative-memory-management`.
- **When**: any page whose content wrapper's class list contains a token *ending*
  in `toc` (`has-toc`, `no-toc`, `sidebar-toc`…).
- **Why**: `main.go:51` `(?i)(?:^|[\s_-])(?:toc|…)(?:$|[\s_-])` matches `has-toc`
  (a `-` before `toc` satisfies the boundary), and `main.go:336-340` calls
  `Remove()` on the element and its subtree: 12,316 of 12,384 words deleted.
- **How to reproduce**: `<div class="page-html has-toc">…article…</div>` → `wr --md` prints only the title.
- **Test**: `TestQAExtraction/QA-F-001_has_toc_token`.
- **Fix**: match whole class tokens against a set (`{"toc","table-of-contents","breadcrumbs","sidebar","cookie…"}`) instead of the boundary regex; also guard element size (never remove an element holding >50% of the root text).
- **Resolución**: `junkClass` regex replaced by token matching (`hasJunkClass`, `extract.go`): a class token is junk if it equals a junk word or *starts* with one followed by `-`/`_` (`toc-wrapper`, `sidebar__left`); a word at the *end* of a token (`has-toc`, `no-sidebar`) is a layout flag and no longer matches. Size guard added: an element holding more than 50% of the root's words is never removed. Trade-off: `left-sidebar`/`main-sidebar` style classes are no longer matched by name (they are usually `<aside>` anyway); a flag-suffix rule would have risked content loss again.
- **Effort**: S · **Risk**: the word "toc" is removed from content words like "toc" in prose (regression to watch: `TestQAExtraction/sidebar_junk_removed`).

### QA-F-002 · Root = the first `<article>`

- **State**: Resuelto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: phoronix.com (8 of 4,929 words), seangoedecke.com (3),
  codeberg.org/forgejo/forgejo, fly.io/blog, charm.sh/blog, elmundo.es (the
  crossword teaser), github.blog category pages.
- **When**: any page with several `<article>` elements: index pages, tag pages,
  search results.
- **Why**: `main.go:321-327` takes `doc.Find("article").First()` with no notion of
  which element holds the reading content.
- **Test**: `TestQAExtraction/QA-F-002_first_article_teaser`.
- **Fix**: score candidates (`article` elements, `main`, `body`) by text length /
  paragraph density / link density and pick the winner; only fall back to the
  first article when no other candidate is meaningfully larger (e.g. ≥5×).
- **Resolución**: `pickRoot` (`extract.go`) weighs the outermost `<article>` elements by word count: one that holds ≥60% of the article text is the root, unless it is under 20% of `<main>`/`<body>` (a banner or lone card); otherwise the whole `<main>`/`<body>` is used so no teaser is picked over the rest. Real pages: phoronix 9→4,117 words, fly.io/blog 69→11,842.
- **Effort**: M · **Risk**: picking a comment thread instead of the article; guard with the paragraph-density signal and keep `<article>` preference on ties.

### QA-F-003 · `dropTags` deletes whole `<form>` elements

- **State**: Resuelto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: `gov.uk/government/news`, `gov.uk/government/publications`
  (5,040 → 0 words).
- **When**: the page wraps its main content in a form (search/filter wrappers).
- **Why**: `main.go:46` includes `form` in `dropTags`; `main.go:332` removes the
  element and everything inside it.
- **Test**: `TestQAExtraction/QA-F-003_form_wrapped_content`.
- **Fix**: drop controls only (`input,select,textarea,button,label`), keep the
  form's remaining content.
- **Resolución**: `form` removed from `dropTags`; only controls go (`input`, `select`, `textarea`, `button`, now also `label`). gov.uk/government/news 6→804 words.
- **Effort**: S · **Risk**: search forms leave stray field labels; mitigate by dropping `label` too.

### QA-F-004 · `<header>` removal eats the page title

- **State**: Resuelto · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: every Wikipedia article (`<h1>` lives in
  `<header class="mw-body-header">`), docs whose `<h1>` is inside `<header>`.
- **When**: the extraction root is not an `<article>` (main/body roots).
- **Why**: `main.go:333-335` removes every `<header>` for non-article roots.
- **Test**: `TestQAExtraction/QA-F-004_header_h1`.
- **Fix**: when a removed `<header>` contains the first `<h1>` of the root, keep it
  (or promote the last heading to the title).
- **Resolución**: `dropHeaders` (`extract.go`): when the first `<h1>` is inside a `<header>`, only its text is re-inserted as the first `<h1>`; the rest of the header (taglines, nav) is still dropped.
- **Effort**: S · **Risk**: site chrome in headers (nav) leaks back; only keep the heading, drop the rest of the header's children.

### QA-F-005 · `<title>` fallback blocked by `#` inside code

- **State**: Resuelto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: Wikipedia `Markdown` and `Python` (`# comment` in samples),
  blog.burntsushi.net/ripgrep (shell comments).
- **When**: the extracted text contains any line starting with `# `, fences
  included.
- **Why**: `main.go:389-391` runs `(?m)^#\s` over the whole Markdown.
- **Test**: `TestQAExtraction/QA-F-005_title_fallback_fence`.
- **Fix**: test for a real title heading outside of code fences, or detect the
  fence blocks and ignore them.
- **Resolución**: `hasTitleHeading` ignores fenced blocks (shared fence scanner with QA-F-006). Wikipedia Markdown/Python now open with their title.
- **Effort**: S · **Risk**: none meaningful; the check only decides whether to prepend `<title>`.

### QA-F-006 · Global regexes rewrite code inside fences

- **State**: Resuelto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: 13 blank-line collapses, 19 trailing-space deletions and 1 deleted
  link-like line across the 308 language samples; reproduced by `cat -A`.
- **When**: any fenced code block containing blank lines, trailing whitespace or a
  line of Markdown links.
- **Why**: `main.go:382-384` applies the prose cleanup (`\n{3,}`, `[ \t]+\n`,
  link-block removal) to the entire Markdown text.
- **Test**: `TestQACodeFidelity/QA-F-006_*`.
- **Fix**: make the post-processing fence-aware (split the document into
  fence/no-fence runs, or apply the cleanups with a small state machine) so
  prose normalization never touches code bodies.
- **Resolución**: `mapOutsideFences` (`extract.go`) runs the prose cleanup (`tidyProse`) only outside fences, including fences indented inside list items. Code is verbatim: blank lines, trailing spaces and link-like lines.
- **Effort**: M · **Risk**: prose pages get slightly noisier; assert with the existing render tests.

### QA-F-007 · `sanitize()` strips `ESC` inside code

- **State**: Resuelto · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: any code sample containing real escape bytes (terminal tutorials,
  tput examples).
- **When**: `\x1b` appears inside a fence: `main.go:387` → `sanitize()` at
  `main.go:395-399` removes it (`\x1b[31m` renders as `[31m`).
- **Test**: `TestQACodeFidelity/QA-F-007_escape_bytes_in_code`.
- **Fix**: keep the security property (never let control bytes reach the
  terminal) but restore `ESC` inside code blocks in the same way wr already
  handles them elsewhere, or replace them with a visible placeholder
  (`␛` / `^[`) that survives the pipeline unchanged.
- **Resolución**: ESC bytes inside `<pre>` become the visible symbol `␛` (U+241B) before conversion; `sanitize` is unchanged, so no control byte can reach the terminal from `--md`, the cache or the renderer. **The scenario was wrong and was changed**: its input used a Go raw string, so it contained the four characters `\x1b`, not an ESC, and its assertion (a raw ESC in the Markdown) is exactly what the security property forbids. It now feeds a real ESC and asserts `␛`; a second scenario pins that literal `\x1b` text is untouched.
- **Effort**: M · **Risk**: the terminal injection surface; keep the sanitizer for everything except code (the renderer draws code lines itself).

### QA-F-008 · Language detection recognizes too few conventions

- **State**: Resuelto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: 1,544 of 6,696 fences carry no language on docs sites (77%): zig 831,
  nim 442, GNU make 397, perldoc 354, pkg.go.dev 278, bash manual 178, Rails 150,
  numpy 138 (`highlight-ipython`), Godot 123 (`highlight-gdscript`), Django 120
  (`highlight-sql`), pandas 80, pytest, Solidity.
- **When**: the site uses Sphinx/Pygments (`highlight-*`), MediaWiki
  (`mw-highlight-lang-*`), or no class at all.
- **Why**: `main.go:47-52` + `main.go:281-312` only read `language-*`,
  `highlight-source-*`, `brush:*` and `data-language`.
- **Test**: `TestQALanguageDetection/*`.
- **Fix**: extend the patterns (`highlight-<lang>`, `mw-highlight-lang-<lang>`,
  `language-*`, `prettyprint lang`), extend `langMap` (`cpp`, `cs`, `pycon`,
  `ipython`…); leave unlabeled blocks as `text` (documented) rather than guessing
  wrongly.
- **Resolución**: Sphinx/Pygments `highlight-<lang>` and MediaWiki `mw-highlight-lang-<lang>` are read as *weak* conventions, accepted only when chroma has a lexer of that name (`weakLangRes`, `langOf`); `highlight-none`/`-default`/`-text` mean no language. `langMap` gained `ipython`, `ipython3`, `python3`, `py3`, `pycon`, `cs`, `nodejs`. Strong conventions (`language-*`, …) still win over a wrapper. **Limit**: Sphinx `highlight-default` means "the project's default language" and the page does not say which (numpy: 123 of 138 blocks), so those stay `text` by the no-guessing principle; bare `<pre>` with no class is the same. The ≥70% target of the plan cannot be met by class conventions alone.
- **Effort**: M · **Risk**: wrong guessing is worse than no color: prefer a precise map; optional heuristic must be labeled as such.

### QA-F-009 · Tables die when a cell contains a list (or spans)

- **State**: Resuelto · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: 52% of pages that have `<table>` produce no table at all; Wikipedia
  infoboxes are the most visible casualty.
- **When**: any `<td>` with `<ul>/<ol>`, and column/row spans.
- **Why**: the html-to-markdown v2 table plugin gives up when a cell is not
  plain inline text; colspan/rowspan are flattened silently and headerless
  tables gain a fake empty header row.
- **Test**: `TestQATables/*`.
- **Fix**: pre-normalize the HTML for the converter (flatten cell lists into
  ` · `-separated text) or implement the table conversion in `toMarkdown`;
  respect spans when possible, otherwise render as structured text.
- **Resolución**: `flattenTables` (`extract.go`) normalizes the DOM before the converter: lists in cells become `a · b`, `<p>`/`<div>`/`<br>` in cells become spaces, `colspan`/`rowspan` become real (empty) cells, and a table with no header row promotes its first row instead of getting an empty fake one. Tables containing nested tables are left alone. Wikipedia's infobox is now a table.
- **Effort**: L · **Risk**: regression on tables that already work — keep `TestQATables/table_data_kept` and the existing render tests green.

### QA-F-010 · UTF-8 without a declared charset decodes as windows-1252

- **State**: Resuelto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: pages and local files whose first non-ASCII byte is past the ~1 KB
  sniff window (`charset-long.html` repro; affected chroma samples: arturo,
  modelica, gherkin).
- **When**: HTML with no BOM and no `<meta charset>`, accents/deviations late in
  the document.
- **Why**: `main.go:270-276` → `charset.NewReader` defaults to windows-1252 when
  the detect window has no declaration.
- **Test**: `TestQACharset/QA-F-010_utf8_past_sniff_window`.
- **Fix**: try UTF-8 first (validate the whole document), fall back to
  windows-1252; or sniff a wider window. Applies to `file://` and `http(s)://`
  alike.
- **Resolución**: `fetch` reads the whole body and `toUTF8` decides: a charset from the header, a BOM or `<meta charset>` is honored; if nothing declares one, a body that is valid UTF-8 (ignoring a character cut by the 8 MB cap) is taken as UTF-8, otherwise windows-1252. Applies to files and URLs.
- **Effort**: S · **Risk**: none for valid UTF-8; keep the BOM path.

### QA-F-011 · Tabs expand to four spaces

- **State**: Caracterización · **Baja** · **Detectado en**: v0.2.1
- **When**: displaying and copying code (`render.go:542`).
- **Decision**: keep as-is (documented) or make it a setting; the test asserts
  the current behavior so a change is explicit.
- **Test**: `TestQAClipboard/code_tabs_become_four_spaces` (asserts 4 spaces).

### QA-F-012 · Footnote link lists dominate long pages

- **State**: Abierto (causa corregida, decisión de producto pendiente) · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: —
- **Where**: Wikipedia articles 40–47% of rendered lines, HN front page 58%,
  jvns.ca 56%, 13% of all lines globally.
- **When**: pages with hundreds of links and the default `links = "footnotes"`.
- **Why**: every *distinct* link becomes a numbered note (`render.go:170`). The
  original text said duplicates were not collapsed; that was wrong: repeated
  URLs already share one number (`linkIdx`, `render.go:782`), now pinned by
  `TestQALinkModes/footnotes_reuse_the_number_of_a_repeated_url`. The length comes
  from pages with hundreds of different links.
- **Test**: `TestQALinkModes/*` (all three modes stay correct).
- **Fix**: collapse repeated URLs, cap the list, or switch the default to
  `inline`/`hidden` for pages above N links.
- **Effort**: M · **Risk**: links must stay clickable and the count visible.

### QA-F-013 · Non-semantic wrappers are not dropped

- **State**: Resuelto · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: v0.2.1 (pending release, PR `fix/qa-findings`)
- **Where**: docs.python.org ("### Navigation" first), pkg.go.dev (133
  "cookie" mentions), eldiario.es (80 social links), HN/lobsters (comments),
  news homepages (subscribes).
- **When**: chrome implemented as `<div role="navigation">`, `.related`,
  `.newsletter`, `.social` instead of `nav/aside/footer`.
- **Why**: `dropTags` only removes semantic elements; `junkClass` only knows four
  class words.
- **Test**: `TestQAExtraction/QA-F-013_non_semantic_nav`.
- **Fix**: drop `[role=navigation]`, `[role=banner]`, `[role=contentinfo]`;
  extend `junkClass` (with exact-token matching, see QA-F-001) to
  `related|newsletter|social|share|cookie|comment|banner|advert`.
- **Resolución**: `[role=navigation|banner|contentinfo]` are dropped, and `junkWords` now also covers `related`, `newsletter`, `social`, `share`, `sharing`, `advert`, `advertisement` (same token rules and size guard as QA-F-001; the role drop has no size guard). **Not added on purpose**: `comment(s)` and `banner` classes — on Hacker News/lobsters the comments *are* the page, and `banner` often wraps a hero with the `<h1>`.
- **Effort**: M/L · **Risk**: aggressive class removal can delete content (QA-F-001): require exact tokens + size guard.

---

## External limits, not defects

No ID assigned: they are not `wr`'s code.

| Limit | Examples from the round |
|---|---|
| Paywall / bot wall (401/403) | nytimes.com, wsj.com, economist.com, ft.com, elpais.com, reuters.com, apnews.com, Stack Exchange, state.gov, nih.gov, science.org |
| JavaScript-rendered content | xeiaso.net (Cloudflare challenge), openlibrary.org (verification), go.dev/tour, notion.so, x.com, instagram.com |
| TLS / HTTP/2 server-side | barrapunto.com (certificate), washingtonpost.com (stream error), semanticscholar.org & amazon.es (202+EOF) |
| Unsupported formats by design | PDFs, images (clear message, `x` opens the browser) |

---

## v0.2.2 — round 2026-10-09 (build `a4a1102`)

Re-audit after the fix: 537 URLs (263 + 113 + 86 + 75 new) plus 320 language
entries, hostile offline fixtures, a local HTTP micro-server, a PTY smoke of the
TUI and the clipboard matrix. Full report: [`QA-v2.md`](QA-v2.md).

Verdict: the twelve fixed findings hold on their original pages (fasterthanli
0 → 9,249 words, gov.uk/news 0 → 1,161, phoronix 8 → 5,886, fly.io/blog
69 → 13,781, Wikipedia keeps its title and infobox, no mojibake in 421 pages),
and the code body stays byte-identical except for documented wrapping and tabs.
QA-F-012 stays open. The findings below come from this round and are open: each
one has a scenario in `TestQARound2` that asserts the desired behavior, so
`WR_QA_STRICT=1` is red on them until they are fixed.

| ID | State | Severity | Area | Summary |
|---|---|---|---|---|
| QA-F-014 | Abierto | Media | Code | Line numbers are glued to the code (Torchlight `div.line` > `span.line-number`) |
| QA-F-015 | Abierto | Media | Code | The last blank line of a code block is lost (`fence()` trims trailing newlines) |
| QA-F-016 | Abierto | Media | Extraction | A line-number gutter **cell** outside `<pre>` leaks as stray text |
| QA-F-021 | Abierto | Media | Extraction | A leading breadcrumb/nav strip survives as the first line of content (pages without `<h1>`) |
| QA-F-022 | Abierto | Baja | Convert | An HTML comment between two block lists leaks into the Markdown |
| QA-F-023 | Abierto | Baja | CLI | `--md -L` keeps links written as `[\[image: …\]](url)` |

Documented in [`QA-v2.md`](QA-v2.md) without an ID (no automatizable scenario
yet): two files given to `wr` are treated as a web search, the title rule always
spans 100 columns (overflow on very narrow terminals), an HTTP/1.0 answer with
`Transfer-Encoding: chunked` shows the chunk sizes, `[¶]`/private-use glyphs and
GNU manuals' `Next:/Previous:/Up:` strips stay in headings, and `[^1]` footnotes
render literally.

---

## Template for the next round

```markdown
## vNEXT — round DATE (build `commit`)
Headline numbers: …

| ID | State | Severity | Area | Summary |
|---|---|---|---|---|
```
