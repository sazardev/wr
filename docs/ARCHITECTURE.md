# Architecture and the test/performance scenario

This document is the map of the whole project: what the code does, where each
responsibility lives, and how every claim about it is checked — unit tests, the
QA corpora, the eval contract, the MCP protocol smoke, and the benchmarks.
Read this first when you arrive at the codebase, and check the last section
before you change something big.

## The pipeline

```
target ─► normalizeInput ─► load ─► toMarkdown ─► renderDoc ─► model ─► screen
(url |      (nav.go)       (fetch)  (convert+       (render.go) (tui.go)  (View/render)
 file |                       │       extract)         ▲
 words)                      │           │            └─ guessLang (guess.go)
                             │           └─ a page drawn by scripts? ─► alternate text
                             │              version ─► headless render (headless.go)
                             └─ cache (~/.cache/wr)      store (~/.local/state/wr)

              model ◄── update check (update.go): GitHub releases, interactive reader only
```

1. **Resolve.** `normalizeInput(in, engine)` (`nav.go`) decides what an entry
   is: an `http(s)://`/`file://` URL, a local path (always `file://`), a
   `looksLikeHost` shorthand, or search words through the configured engine
   (`engineTemplate`, duckduckgo/wikipedia/github or a custom `%s` URL).
2. **Load.** `load(src)` → `loadFrom(src, fetchURL)` (`fetch.go`; `loadWith` is
   the same with a plain `fetcher` injected, which is why extraction is
   testable without a network): fetch (20 s timeout, one retry on a reset or a
   timeout, 8 MB cap, UA fixed, `Accept-Language` from the locale, TLS
   verified, charset → UTF-8) and report the **final address after redirects**,
   which is what relative links are resolved against. An HTTP error names who
   answered (the `<title>` of the error page) and what to do. Then `convert`
   dispatches by media type: HTML → `toMarkdown`, Markdown passes through
   `sanitize`, JSON/XML/plain is fenced. A page that came out as a *JS shell*
   (see `shell.go`) is retried in order: its advertised text version
   (`alternateLink`), then `renderedMarkdown` (`headless.go`: an installed
   Chromium/Chrome run headless, its own profile, the whole process group
   killed on timeout, output capped, skipped when `render_js` is off), else the
   page's own summary under a notice. On success it writes the cache when it is
   an http page, except for a bare shell.
3. **Extract.** `toMarkdown` (`convert.go`): `collectHints` first saves what a
   shell page says outside its body (`<noscript>`, description, JSON-LD); then
   `pickRoot` scores `<article>`/`<main>`/`<body>` by text weight, `dropTags`
   removes `<form>` and friends, `dropHidden` what the page hides,
   `dropHeaders`/`dropChrome` strip the page chrome (with a 50% size guard so a
   chrome-shaped document is not emptied), `flattenCodeLines` turns block-line
   code (Expressive Code) into text, code blocks keep their language
   (`language-*`, `data-language`, Sphinx `highlight-*`, MediaWiki
   `mw-highlight-lang-*`), tables are flattened, headings lose their permalink
   anchors (`cleanHeadings`), anchors are tidied (`tidyAnchors`: empty and skip
   links go, cards keep the title as the link) and absolutized, then
   html-to-markdown v2 plus fence-aware cleanups (`tidyMarkdown`,
   `dedupeLinkLines`, `dropLeadingCrumbs`) and `sanitize` (control characters,
   including ESC, are removed — page content is untrusted).
4. **Render.** `renderDoc(md, width, cfg)` (`render.go`) parses the Markdown
   with goldmark, highlights code with chroma (memoized per block, one
   goroutine per block; a block with no language gets `guessLang`'s answer when
   it is sure, marked `?` internally and labelled "(guessed)"), and emits self-contained ANSI lines in a `Doc`:
   `Lines`, `Plain`, `Heads`, `Links` (as `LinkSpan` + `URLs`), per-line
   `Meta` for selection. Invisible APC markers placed during rendering are
   scanned into link spans afterwards, so links keep their screen positions.
5. **Interact.** The Bubble Tea v2 model (tui.go): keys, mouse, resize, spring
   scrolling (harmonica), search, selection, toasts, panels (the update window
   is one more: `update_ui.go`). Navigation
   (`browse.go`) keeps `back`/`fwd` stacks of `navEntry` and runs every load
   through `startNav`/`loadCmd`; an overtaken load is dropped by id, and a
   failed load never replaces the page on screen. `newModel` + `Update` +
   `render()` work with no terminal at all — that single property is what makes
   the deterministic UI tests and the agent sessions in `mcp_session.go`
   possible.
6. **Update.** `startUpdateCheck` runs once at launch in the interactive reader
   only (`updateEnabled` is switched on by `runTUI`, so tests, pipes, MCP and
   eval never reach the network for it). `checkForUpdate` trusts the last
   answer for 12 hours, remembers it in `update.json`, and reports a newer
   release, whether it was already announced, and, on the first launch after
   an update, that release's notes. `applyUpdate` downloads the archive from
   the project's release URL only, verifies its SHA-256 against
   `checksums.txt`, runs the new binary (`--version` must match the tag) and
   renames it over the running one.

## Where things live

One `package main` at the root (the reader), plus the docs site and the
tooling. Nothing in the reader is exported and there is no `internal/`: the
server modes (`--mcp`, `--eval`) are files in the same package on purpose, so
they use `load`, `newModel` and the store directly without a public API to
maintain.

| Path | What |
|---|---|
| `main.go` | CLI entry: flags → config → `runTUI`, `--md`/pipe output (`NO_COLOR` aware), `--update`, `--mcp`, `--eval` |
| `cli.go` | the flag table (`parseArgs` is driven by `flagDefs`) |
| `config.go` | `Config`, defaults, commented TOML load/save, `normalize` |
| `fetch.go` | `fetchURL` (final URL, retry, readable errors), `load`, `loadFrom`, `loadWith`, charset → UTF-8 |
| `convert.go` | media-type dispatch, HTML → Markdown, `sanitize`, `stripLinks` |
| `extract.go` | root picking, chrome and hidden dropping, headings, anchors/cards, code lines, table flattening, crumbs, dedupe |
| `shell.go` | JS-shell detection, the page's own summary (`collectHints`), the notice, advertised text versions |
| `headless.go`, `headless_unix.go`, `headless_windows.go` | rendering a shell page with an installed Chromium/Chrome (process group, cap, counter for the UI) |
| `guess.go` | the conservative, labelled code-language guesser |
| `update.go`, `update_ui.go` | the update check, the verified self-update, the release notes; the update window, footer chip and menu entry |
| `render.go` | Markdown → ANSI, `Doc`, `LinkSpan`, link positions |
| `nav.go` | input resolution, link resolution, the start page |
| `browse.go` | navigation, links/focus/hints, address bar, history and bookmark panels |
| `tui.go` | the model: `Update`, `View`/`render`, geometry, scroll, footer, toasts |
| `keys.go` | actions, default bindings, keymap build/override |
| `search.go` | in-page search (smart-case) and highlight spans |
| `selection.go` | mouse selection, OSC 52 + external clipboard |
| `overlays.go`, `settings.go`, `theme.go`, `style.go`, `anim.go`, `braille.go` | panels, live settings, palette, SGR, springs, glyphs |
| `store.go`, `cache.go` | history/bookmarks JSON; the sha256-keyed Markdown cache |
| `mcp*.go`, `eval.go` | the agent harness: MCP server, headless sessions, eval scenarios |
| `web/` | the docs site (Astro, separate build) |
| `docs/` | guide, design system, QA reports, benchmarks, this file |
| `scripts/` | QA corpora and probes, `bench.sh` + `bench_compare.py`, release, hooks |

### State on disk

| What | Path | Format |
|---|---|---|
| config | `~/.config/wr/config.toml` (`os.UserConfigDir`) | commented TOML, atomic write, 0644 |
| cache | `~/.cache/wr/` (`os.UserCacheDir`) | one file per page: `wr-cache 2 <unix> <src>` + Markdown, 0600 |
| history | `~/.local/state/wr/history.jsonl` (`XDG_STATE_HOME`) | one JSON object per visit (`{"u","t","a"}`), 0600 |
| bookmarks | `~/.local/state/wr/bookmarks.json` | JSON array, 0600 |
| update memory | `~/.local/state/wr/update.json` | `{checked, failed, latest, notified, seen}`, 0600 |

State is hand-rolled XDG (`XDG_STATE_HOME`), config and cache use Go's
`os.User*Dir` (which honor `XDG_CONFIG_HOME`/`XDG_CACHE_HOME` on Linux).
Directories are 0700. Every per-user path is read lazily per call, so tests
and benchmarks redirect all three with environment variables and nothing else
(`isolateAll` in tui_test.go, `benchIsolate` in bench_test.go).

## How everything is checked

| Surface | Covers | Run |
|---|---|---|
| Unit tests (`*_test.go`) | renderer + link positions, search, selection/copy, cache, config, keys, store, navigation (click, back/forward, failures, overtaken loads), extraction scenarios, JS shells and the headless render (a fake browser script, including a timeout that must kill its children), the language guesser, the whole update path (a stub release API, a checksummed archive, refusal of corrupt, mismatched or foreign downloads) and the update window | `go test -race ./...` |
| Extraction audit (`scripts/qa/audit.py`) | 70+ real developer-docs URLs through a binary: backslash breaks, empty links, repeated lines, thin pages, shells, and the opening lines of each page (QA round 3) | `WR=./wr python3 scripts/qa/audit.py --tops` |
| QA corpora (`scripts/qa/`) | 460+ real URLs in 10 categories, chroma lexers, synthetic cases, PTY copy — output in `qa-out/` | `CORPUS=corpus python3 scripts/qa/harness.py` etc. |
| QA scenario tests (`qa_scenarios_test.go`) | every finding in `docs/QA-FINDINGS.md` has a test; a meta-test keeps the three in sync | `go test -race ./...` (`WR_QA_STRICT=1` asserts open findings) |
| `--eval` | 9 deterministic offline scenarios (input, extraction, render, links, session navigation, search, store, cache, config) as JSON, exit 1 on failure | `go run . --eval` |
| MCP smoke (`scripts/qa/mcp_smoke.py`) | the agent protocol end to end against any binary: handshake, tools/list, reading tools, a full session, config | `WR=./wr python3 scripts/qa/mcp_smoke.py` |
| Benchmarks (`bench_test.go`) | extraction, render, load path (cache vs no cache), interaction, store, search — local, deterministic, with allocations | `go test -run '^$' -bench . -benchmem` |

Two properties hold across the automated ones: **no network** (loopback
`httptest` stands in for the web, for the release API and for the release
assets; a shell script stands in for Chromium) and **no wall-clock dependence**
(animations off, synthetic pages). The audit and the QA corpora are the
exceptions on purpose: they exist to meet the real web. CI runs gofmt, `go mod tidy -diff`, `go vet`, `go test -race ./...`
and `go build ./...`; the git hooks repeat the fast ones on commit and push.

## The performance scenario

The measured baseline, the hot paths and the bottlenecks live in
[docs/BENCHMARKS.md](BENCHMARKS.md). The short version after the optimization
pass: extraction was the slowest stage (~4 MB/s on a 300 KB article, mostly the
Markdown writer and our own regexp cleanups) and is now ~30–37% faster with a
one-pass cleaner; in-page search dropped 48% (and 42% end to end, allocations
x100 down) once the matcher stopped using the regexp engine; renders are ~15%
faster since `Plain` stopped paying for the full ANSI parser; the cache turns a
15.8 ms miss into a 2.1 ms hit; and the interactive path is ~100 ns per key
with zero allocations. The budget every change should keep:

- a keypress stays in the microsecond range and allocates nothing;
- a cached page open stays a single-digit millisecond;
- no benchmark regresses more than 25% in ns/op (`scripts/bench.sh compare`).

## Changing something big

The order that keeps the claims honest:

```sh
sh scripts/bench.sh baseline     # 1. capture before
# 2. make the change, with tests
go test -race ./...              # 3. the whole suite still green
go run . --eval                  # 4. the contract still holds
WR=$PWD/wr python3 scripts/qa/mcp_smoke.py   # 5. the protocol still answers
sh scripts/bench.sh compare      # 6. no regression over the threshold
sh scripts/bench.sh profile X    # 7. profiles if you touched a hot path
```

Commits are Conventional Commits (releases are cut by release-please from
them), and the hook installed by `scripts/setup-hooks.sh` runs gofmt, vet and
the tests on commit. Export nothing from `package main` unless a caller
outside it truly needs it; prefer another mode of the binary (like `--mcp`)
over a new public API.
