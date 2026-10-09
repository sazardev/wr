# QA findings register

Versioned record of every finding from the QA rounds documented in
[`QA.md`](QA.md). This file is the source of truth for **what is open**;
[`QA-PLAN.md`](QA-PLAN.md) is the source of truth for **what to do and in what
order**, and `qa_scenarios_test.go` is the automatic test mode that covers each
finding.

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
| QA-F-001 | Abierto | Bloqueante | Extraction | `junkClass` matches *substrings*: `class="… has-toc"` deletes the whole page |
| QA-F-002 | Abierto | Alta | Extraction | Root = first `<article>`: only the first teaser survives listings |
| QA-F-003 | Abierto | Alta | Extraction | `dropTags` deletes whole `<form>` elements (gov.uk news listing → 0 words) |
| QA-F-004 | Abierto | Media | Extraction | `<header>` removed when the root is not `<article>`: Wikipedia's title disappears |
| QA-F-005 | Abierto | Alta | Extraction | `<title>` fallback blocked by any `# ` line, including inside code fences |
| QA-F-006 | Abierto | Alta | Code | Global regexes rewrite code inside fences: blank lines collapse, trailing spaces and link-like lines are deleted |
| QA-F-007 | Abierto | Media | Code | `sanitize()` strips real `ESC` bytes from code blocks too |
| QA-F-008 | Abierto | Alta | Code | Language detection knows 4 conventions; Sphinx `highlight-*`, MediaWiki `mw-highlight-lang-*` and plain `<pre>` are missed |
| QA-F-009 | Abierto | Media | Code | A list (or colspan) inside a table cell destroys the table (html-to-markdown) |
| QA-F-010 | Abierto | Alta | Charset | UTF-8 without a declared charset decodes as windows-1252 once the first non-ASCII byte is past the sniff window (~1 KB) → mojibake |
| QA-F-011 | Caracterización | Baja | Code | Tabs are expanded to 4 spaces for display and copy (asserted, documented) |
| QA-F-012 | Abierto | Media | UX | `links = "footnotes"` dominates long pages (13% of all rendered lines; 40–47% on Wikipedia) |
| QA-F-013 | Abierto | Media | Extraction | Non-semantic wrappers (`role=navigation`, "related" blocks, cookie banners) are not dropped |

### QA-F-001 · `junkClass` matches substrings of class names

- **State**: Abierto · **Severity**: Bloqueante · **Detectado en**: v0.2.1 (`fcb54cf`) · **Resuelto en**: —
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
- **Effort**: S · **Risk**: the word "toc" is removed from content words like "toc" in prose (regression to watch: `TestQAExtraction/sidebar_junk_removed`).

### QA-F-002 · Root = the first `<article>`

- **State**: Abierto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: —
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
- **Effort**: M · **Risk**: picking a comment thread instead of the article; guard with the paragraph-density signal and keep `<article>` preference on ties.

### QA-F-003 · `dropTags` deletes whole `<form>` elements

- **State**: Abierto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: —
- **Where**: `gov.uk/government/news`, `gov.uk/government/publications`
  (5,040 → 0 words).
- **When**: the page wraps its main content in a form (search/filter wrappers).
- **Why**: `main.go:46` includes `form` in `dropTags`; `main.go:332` removes the
  element and everything inside it.
- **Test**: `TestQAExtraction/QA-F-003_form_wrapped_content`.
- **Fix**: drop controls only (`input,select,textarea,button,label`), keep the
  form's remaining content.
- **Effort**: S · **Risk**: search forms leave stray field labels; mitigate by dropping `label` too.

### QA-F-004 · `<header>` removal eats the page title

- **State**: Abierto · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: —
- **Where**: every Wikipedia article (`<h1>` lives in
  `<header class="mw-body-header">`), docs whose `<h1>` is inside `<header>`.
- **When**: the extraction root is not an `<article>` (main/body roots).
- **Why**: `main.go:333-335` removes every `<header>` for non-article roots.
- **Test**: `TestQAExtraction/QA-F-004_header_h1`.
- **Fix**: when a removed `<header>` contains the first `<h1>` of the root, keep it
  (or promote the last heading to the title).
- **Effort**: S · **Risk**: site chrome in headers (nav) leaks back; only keep the heading, drop the rest of the header's children.

### QA-F-005 · `<title>` fallback blocked by `#` inside code

- **State**: Abierto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: —
- **Where**: Wikipedia `Markdown` and `Python` (`# comment` in samples),
  blog.burntsushi.net/ripgrep (shell comments).
- **When**: the extracted text contains any line starting with `# `, fences
  included.
- **Why**: `main.go:389-391` runs `(?m)^#\s` over the whole Markdown.
- **Test**: `TestQAExtraction/QA-F-005_title_fallback_fence`.
- **Fix**: test for a real title heading outside of code fences, or detect the
  fence blocks and ignore them.
- **Effort**: S · **Risk**: none meaningful; the check only decides whether to prepend `<title>`.

### QA-F-006 · Global regexes rewrite code inside fences

- **State**: Abierto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: —
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
- **Effort**: M · **Risk**: prose pages get slightly noisier; assert with the existing render tests.

### QA-F-007 · `sanitize()` strips `ESC` inside code

- **State**: Abierto · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: —
- **Where**: any code sample containing real escape bytes (terminal tutorials,
  tput examples).
- **When**: `\x1b` appears inside a fence: `main.go:387` → `sanitize()` at
  `main.go:395-399` removes it (`\x1b[31m` renders as `[31m`).
- **Test**: `TestQACodeFidelity/QA-F-007_escape_bytes_in_code`.
- **Fix**: keep the security property (never let control bytes reach the
  terminal) but restore `ESC` inside code blocks in the same way wr already
  handles them elsewhere, or replace them with a visible placeholder
  (`␛` / `^[`) that survives the pipeline unchanged.
- **Effort**: M · **Risk**: the terminal injection surface; keep the sanitizer for everything except code (the renderer draws code lines itself).

### QA-F-008 · Language detection recognizes too few conventions

- **State**: Abierto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: —
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
- **Effort**: M · **Risk**: wrong guessing is worse than no color: prefer a precise map; optional heuristic must be labeled as such.

### QA-F-009 · Tables die when a cell contains a list (or spans)

- **State**: Abierto · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: —
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
- **Effort**: L · **Risk**: regression on tables that already work — keep `TestQATables/table_data_kept` and the existing render tests green.

### QA-F-010 · UTF-8 without a declared charset decodes as windows-1252

- **State**: Abierto · **Alta** · **Detectado en**: v0.2.1 · **Resuelto en**: —
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
- **Effort**: S · **Risk**: none for valid UTF-8; keep the BOM path.

### QA-F-011 · Tabs expand to four spaces

- **State**: Caracterización · **Baja** · **Detectado en**: v0.2.1
- **When**: displaying and copying code (`render.go:542`).
- **Decision**: keep as-is (documented) or make it a setting; the test asserts
  the current behavior so a change is explicit.
- **Test**: `TestQAClipboard/code_tabs_become_four_spaces` (asserts 4 spaces).

### QA-F-012 · Footnote link lists dominate long pages

- **State**: Abierto · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: —
- **Where**: Wikipedia articles 40–47% of rendered lines, HN front page 58%,
  jvns.ca 56%, 13% of all lines globally.
- **When**: pages with hundreds of links and the default `links = "footnotes"`.
- **Why**: every link becomes a numbered note (`render.go:170`) and duplicates
  are not collapsed.
- **Test**: `TestQALinkModes/*` (all three modes stay correct).
- **Fix**: collapse repeated URLs, cap the list, or switch the default to
  `inline`/`hidden` for pages above N links.
- **Effort**: M · **Risk**: links must stay clickable and the count visible.

### QA-F-013 · Non-semantic wrappers are not dropped

- **State**: Abierto · **Media** · **Detectado en**: v0.2.1 · **Resuelto en**: —
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

## Template for the next round

```markdown
## vNEXT — round DATE (build `commit`)
Headline numbers: …

| ID | State | Severity | Area | Summary |
|---|---|---|---|---|
```
