# QA report v2: validación del fix y ronda agresiva

Status: second audit of the fixes landed after round 1 (see [`QA.md`](QA.md) and
[`QA-FINDINGS.md`](QA-FINDINGS.md)). Round 1 described `fcb54cf`; this one ran
against `a4a1102` (`wr v0.2.2-0.20261009184245-a4a110250e7c`, after the
`fix/qa-findings` refactor into `extract.go` / `fetch.go` / `convert.go`).

- [When](#when)
- [Verdict per finding](#verdict-per-finding)
- [Metrics: round 1 → round 2](#metrics-round-1--round-2)
- [New failures (this round)](#new-failures-this-round)
- [Not failures: what was misjudged](#not-failures-what-was-misjudged)
- [External limits re-verified](#external-limits-re-verified)
- [How to reproduce](#how-to-reproduce)

## When

- Round 2 (fix validation): 2026-10-09, 13:00–20:00 (America/Los_Angeles).
- Environment: Linux (CachyOS), Go 1.27.1, `wr` built from `a4a1102` with
  `go build -ldflags="-s -w"`. Tests ran with an isolated `HOME`; the cache is
  never read in pipe mode (`wr --md URL`, `wr doc.md | cat` always fetch fresh).

### Scope of this round

| Round | Input | Count |
|---|---|---|
| 1 | Front pages/docs (corpus) | 263 |
| 1b | Real articles (corpus2) | 113 |
| 2c | Language/framework docs (corpus3) | 86 |
| **4 (new)** | **Fresh URLs, none seen in rounds 1–2c** (blogs, docs, wiki, news EN/ES, science, forums, gov, code hosts, odd formats, JS walls) | **75** |
| 5 (new) | Offline hostile fixtures: SVG/template/iframe, nested fences, 4-backtick fences, trailing whitespace, HTML comments, entities, CRLF, 4 MB/8 MB documents | 30+ |
| 6 (new) | Language matrix (297 chroma lexers + aliases + edge cases) | 320 |
| 7 (new) | Local HTTP micro-server: gzip, latin-1 with/without meta, chunked, missing Content-Type, redirect to PDF/PNG, HTTP 500, empty body, 3 s slow | 10 |
| 8 (new) | TUI smoke in a PTY: scroll, page, help, search, settings, G/g, Tab+Enter link follow, hints, Ctrl-C | 11 sessions |
| 9 | Clipboard in PTY (drag / double / triple click, wrapped line) | 4 |
| 10 | `go test ./...` and `WR_QA_STRICT=1 go test -run TestQA ./...` (78 QA subtests) | 2 |

Total: 537 URLs, 200+ domains, plus the offline matrix; every URL captured with
`curl` (original), `wr --md` and the piped render, as in round 1.

## Verdict per finding

All twelve fixed findings from the register were re-tested on the exact pages
that motivated them (numbers are this round's `--md` word counts).

| ID | Verdict | Evidence in this round |
|---|---|---|
| QA-F-001 `junkClass` substrings (`has-toc`) | **Validado** | fasterthanli `declarative-memory-management` 0 → **9,249** words; w3.org/TR/html52 (advert redirect stub) → **24,600**; layout flags (`has-toc`) no longer match (`extract.go:110` token rule) |
| QA-F-002 first `<article>` root | **Validado** | phoronix.com 8 → **5,886**; fly.io/blog 69 → **13,781**; codeberg/forgejo 5 → **4,518**; charm.sh/blog → **811**; seangoedecke → **374**; elmundo.es → **3,264**; listings keep every teaser |
| QA-F-003 `<form>` deletion | **Validado** | gov.uk/government/news 0 → **1,161** words; /publications 0 → **495**; form controls still dropped (`convert.go:23`) |
| QA-F-004 `<header>` eats the title | **Validado** | every Wikipedia article keeps its `<h1>`; # Markdown now opens with `# Markdown` (before: no title) |
| QA-F-005 `<title>` blocked by `#` in code | **Validado** | Wikipedia Markdown/Python articles open with their title even though their samples start with `#` (`hasTitleHeading` is fence-aware, `extract.go:183`) |
| QA-F-006 prose rewrites code | **Validado** | 320 language samples: blank lines, trailing spaces, tabs and link-like lines survive (`mapOutsideFences`); hostile fixtures with 4-backtick fences, tilde fences and info strings hold |
| QA-F-007 `ESC` inside code | **Validado** | `␛` placeholder stays; sanitizer still strips every other control byte |
| QA-F-008 language conventions | **Validado, ceiling reached** | Sphinx/Pygments/MediaWiki now detected: pandas 80/80 fences labeled, godot 122/123, django 80/120, numpy 6/138. The rest of the corpus uses **bare `<pre>` with no class at all** (see “Not failures”) |
| QA-F-009 tables with lists/spans | **Validado** | Wikipedia infoboxes are real tables (Coffee: 34k words with the infobox intact); `flattenTables` keeps list cells, spans and headerless tables |
| QA-F-010 charset | **Validado** | 0 mojibake over 421 successful pages; latin-1 without `<meta>` decodes right; BOM still honored |
| QA-F-013 non-semantic chrome | **Validado** | python.org/3/tutorial no longer opens with “### Navigation”; godot's whole class tree disappears (`role=navigation`); docs.python.org 2,720 words |
| QA-F-012 footnotes dominate | **Sigue abierto** | unchanged by design; Wikipedia links are still 40–47% of rendered lines (product decision) |

Offline suites: `go test ./...` is green (the round-1 scenarios all pass for
real now — nothing is skipped) and `WR_QA_STRICT=1` adds the six new scenarios
from this round, which are red until they are fixed, exactly as designed.
`TestQAFindingsCovered` keeps the register and the tests in sync.

## Metrics: round 1 → round 2

The headline: the blocking extractions are gone and code fidelity went up, but
the *sentence-recall* numbers look worse. That is the metric, not the reader —
round 2 captures **more** than the sampled root (listings that round 1 cut to one
teaser), and sampling a root and then capturing the whole page scores 0. The
table separates both readings.

| Metric | Round 1 (`fcb54cf`) | Round 2 (`a4a1102`) |
|---|---|---|
| Real articles captured ≥60% | 64% | **50%** (55/109) — 21 of the 54 below the bar are pages that now capture *more* than the sampled root |
| Real articles median coverage | 0.73 | 0.60 (same cause) |
| Pages extracting >1.2× the root | ~0% (round 1 cut listings) | **54%** (125/231) |
| Front pages captured ≥60% | 33% | 21% (same cause; 113 of 182 are over-capture) |
| Rendered Markdown leftovers | 0 in 232/235 | **0 in 421/423** (the 2 leftovers are code samples that *show* Markdown) |
| Fences WITH a language on docs sites | 23% (1,544/6,696) | **32%** (1,416/4,382) — only Sphinx/Pygments/MediaWiki are detectable |
| Fences WITH a language on standalone articles | 66% (80/121) | **66%** (80/121) — unchanged: the 41 unlabeled are bare `<pre>` |
| Rendered code bodies byte-identical | 88% | **92.5%** (296/320); the other 24 are documented `┆` wrap or tabs→4 spaces |
| Code bodies with the last blank line kept | 272/308 (88%, noisier metric) | 309/320 (96.5%) under the same accounting |
| `wr --md` median / p90 / max | 0.51 s / 0.92 s / — | 0.45 s / 0.9 s / 4.6 s |
| render median / p90 | 19 ms / 107 ms | 26 ms / 121 ms |
| New corpus (75 never-before-tested URLs) | — | 59 extracted, 25% at ≥60%, median 0.33 |
| Copy in PTY (drag/double/triple, wrapped line) | all correct | all correct |
| TUI smoke (scroll, help, search, settings, G/g, Tab+Enter link) | — | 11 sessions, no crash, no panic |
| QA subtests (offline) | — | 78 run green; 6 new ones red until fixed (by design) |

Boilerplate leaks stay at median 1: the worst offenders in round 2 are page
*content* that mentions those words (pkg.go.dev “cookie” = the `SetCookie`
docs, godot “comments” = the GDScript tutorial, laravel “register” = routes,
slashdot “comments” = the story's own thread), not chrome.

## New failures (this round)

None of these lose an article; all have a minimal repro. Ordered by severity.
The first six have an asserting scenario in `TestQARound2` (red until fixed) and
are registered in [`QA-FINDINGS.md`](QA-FINDINGS.md); the rest need a CLI, PTY or
HTTP harness before they can be automated, so they stay here as evidence.

| ID | Sev | Area | What fails | Minimal repro |
|---|---|---|---|---|
| QA-F-014 | Media | Code | **Line numbers are glued to the code** when the site marks each line: Laravel (Torchlight) `<div class='line'><span class="line-number">1</span>use …` comes out as `1use Illuminate\Support\Facades\Route;`, and the next line as a stray `\xa0` | `wr --md` on `laravel.com/docs/11.x/routing` (fence `php`) |
| QA-F-015 | Media | Code | **The last blank line of a code block is lost**: `fence()` does `TrimRight(text, "\n")` (`convert.go:70`). 11 of 320 language samples changed that way (J, MLIR, Natural, Odin, Pony, QML, SYSTEMD, Spade, Tal…) | `<article><pre><code>a=1\n\n</code></pre></article>` → trailing empty line gone |
| QA-F-021 | Media | Extraction | **Leading chrome survives as the first line of content**: pages whose `<h1>` is missing open with a breadcrumb/nav strip (`[std](…)::[vec](…)`, `[Read in English] [Edit]`, `- NEWS`) before the first section heading, so the document starts with chrome instead of the article | `wr --md` on `research.swtch.com/generic`, `go.dev/doc/effective_go`, `learn.microsoft.com/.../tour-of-csharp` |
| QA-F-016 | Baja | Code | an unlabeled `<pre>` inside a line-number **table** leaves the gutter cell as stray text: `<td class="line-no">1</td>` (a sibling of the code cell) is not covered by the `pre *` cleanup — `TestQARound2/QA-F-016` | synthetic `code-langs.html` → a lone `1` above the fence |
| QA-F-017 | Baja | CLI | **Two files are treated as a web search**: `wr --md a.html b.html` searches “a.html b.html” and shows an empty page (`wr --md a.html` alone works) | `wr --md h1.html h2.html` |
| QA-F-018 | Baja | Render | **Fixed-width chrome**: the title rule is always 100 columns and tables have a minimum width, so with `COLUMNS=10/20` lines overflow the terminal (prose wraps fine) | `COLUMNS=10 wr doc.md` |
| QA-F-019 | Baja | Network | an **HTTP/1.0 response with `Transfer-Encoding: chunked`** (spec violation) shows the chunk sizes as text (`7 &lt; 7 head&gt;…`). Go already tolerates it; `curl` on the same server decodes it fine | local micro-server |
| QA-F-020 | Baja | Extraction | heading anchors and manual chrome leak: Sphinx/Godot/Ansible `[¶](…)` and private-use glyphs (`\uf0c1`) stay inside headings; GNU manuals' `Next:/Previous:/Up:` strip is `div.header`, which is not a dropped tag | gnu.org bash/make manuals, ansible docs |
| QA-F-022 | Baja | Convert | an **HTML comment between two block lists** is kept as literal text in the Markdown (in other positions it is correctly removed) — `TestQARound2/QA-F-022` | `<ul>…</ul>\n<!--X-->\n<ol>…</ol>` inside `<article>` |
| QA-F-023 | Baja | Markdown | `--md -L` does not strip links written as `[\[image: alt\]](url)`: the regex cannot match the escaped form, so the URL stays in the output — `TestQARound2/QA-F-023` | `wr --md -L page-with-images.md` |
| QA-F-024 | Baja | Markdown | `[^1]` footnotes are shown literally (no footnote syntax support; the note sits at the end of the document as plain text) | `wr note.md` with `[^1]` |

Repros for QA-F-014/015/022 are pure HTML files (no network):

```html
<article><pre><div class='line'><span class="line-number">1</span>use X;</div></pre></article>
```

```sh
wr --md torchlight.html   # "1use X;"
```

## Not failures: what was misjudged

The round-1 metrics are still *sentence sampling against the chosen root*, which
punishes correctly-dropped chrome. These looked like bugs and are not — each was
opened and read:

| Case | Looked like | Reality |
|---|---|---|
| godot step-by-step (19k root words, 1.6k captured) | 91% content loss | the 17k missing words are the class-tree sidebar (`role=navigation`); the article itself is complete, all 12 code blocks labeled |
| dart.dev/language/classes | 15% coverage | 1,355 of 1,798 article words; the rest is the docs drawer |
| bbc.com/mundo topic pages, elmundo election pages, dw.com sections | recall 0.00 | the sampled root was a carousel; `wr` now captures the whole page, which is the QA-F-002 fix working |
| lanacion, smithsonian, github.blog/news-insights | recall 0.00, ratio >1 | same — listings captured in full, the metric samples one teaser |
| godot gdscript (21 “comments”) | boilerplate leak | the GDScript tutorial explains `# comments` |
| pkg.go.dev/net/http (133 “cookie”) | cookie banner leak | the docs of `http.SetCookie` |
| wikipedia Markdown / rfc9110 / raw wikitext | leftover Markdown syntax | code samples *showing* Markdown/ABNF syntax inside a frame |
| aljazeera liveblog (12 words), xeiaso.net, nestjs, kotlinlang, erlang docs, go.dev/tour, reddit/arduino/gitlab | near-empty extraction | JS-rendered pages (see external limits); the title is still rescued |
| 8 MB cap on a 20 MB document | truncation | the documented `maxBytes` cap degrading cleanly |
| `-` as stdin | broken | a lone dash has always been a search word (`main_test.go:21`), not a stdin interface |

## External limits re-verified

Round 2 confirms the same walls, with the same counts, and adds the new corpus
evidence — nothing regressed and nothing new appeared:

- Sites answering 401/403 to any non-browser client: nytimes, wsj, economist,
  ft, elpais, reuters, apnews, medium, iso.org, stackexchange, state.gov, nih,
  science.org, claude.ai, rachelbythebay, git-scm book (now 404).
- JavaScript-rendered: xeiaso.net (Cloudflare), openlibrary.org, go.dev/tour,
  notion.so, x.com, instagram.com, aljazeera liveblog, docs.nestjs.com,
  kotlinlang.org, erlang.org, observablehq.
- Dead URLs this round (15): brendangregg, nelhage, pointersgonewild,
  mazzo.li, jvns 2026 post, lucumr, bbc future, nature s41586, link.springer,
  git.sr.ht/~sircmpwn, bitbucket, swift book, symfony, flutter docs, electronjs.
- TLS/HTTP2 server-side: barrapunto, washingtonpost, semanticscholar, amazon.es.
- Unsupported formats by design: PDF, PNG (clear message, opens in the browser
  with `x`), confirmed also behind a 302 redirect.

Language detection ceiling (verified, not a bug): the blocks that stay `text`
are `<pre>` with **no class at all** — ziglang 831, nim 442, GNU make 397,
perldoc.perl.org 354, pkg.go.dev 278, bash manual 178, Rails guides 150. Those
sites name the language nowhere, and the “never guess” principle (QA-F-008) keeps
them uncolored. The plan's ≥70% target cannot be met by class conventions alone.

## How to reproduce

```sh
go build -ldflags="-s -w" -o wr .

# offline, deterministic
go test ./...
WR_QA_STRICT=1 go test -run TestQA -v ./...   # every finding, no skips

# same harness as round 1, plus the new corpus
export WR=$PWD/wr
for c in corpus corpus2 corpus3 corpus4; do
  CORPUS=$c QA_ROOT=/tmp/qa-$c python3 scripts/qa/harness.py
  QA_ROOT=/tmp/qa-$c python3 scripts/qa/analyze.py
  QA_ROOT=/tmp/qa-$c python3 scripts/qa/metrics.py
done

# language matrix
QA_ROOT=/tmp/qa-langs go run ./scripts/qa/probe/lexers > /tmp/qa-langs/chroma-lexers.tsv
QA_ROOT=/tmp/qa-langs python3 scripts/qa/langs_corpus.py
QA_ROOT=/tmp/qa-langs python3 scripts/qa/langs_run.py

# synthetic + clipboard
QA_ROOT=/tmp/qa-syn python3 scripts/qa/gen_synthetic.py
QA_ROOT=/tmp/qa-syn python3 scripts/qa/run_synthetic.py
QA_ROOT=/tmp/qa-copy python3 scripts/qa/copy_pty.py
```

Round 1's report is [`QA.md`](QA.md); the findings of this round that have an
automatable scenario are registered in [`QA-FINDINGS.md`](QA-FINDINGS.md)
(QA-F-014… QA-F-016, QA-F-021… QA-F-023, all open) and the work order lives in
[`QA-PLAN.md`](QA-PLAN.md) (ola 5).
