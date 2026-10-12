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
 file |                       │       extract)
 words)                      │
                             └─ cache (~/.cache/wr)      store (~/.local/state/wr)
```

1. **Resolve.** `normalizeInput(in, engine)` (nav.go:72) decides what an entry
   is: an `http(s)://`/`file://` URL, a local path (always `file://`), a
   `looksLikeHost` shorthand, or search words through the configured engine
   (`engineTemplate`, duckduckgo/wikipedia/github or a custom `%s` URL).
2. **Load.** `load(src)` → `loadWith(src, fetch)` (fetch.go:28/32): fetch
   (20 s timeout, 8 MB cap, UA fixed, TLS verified, charset → UTF-8), then
   `convert` (convert.go:76) dispatches by media type: HTML → `toMarkdown`,
   Markdown passes through `sanitize`, JSON/XML/plain is fenced. On success it
   writes the cache when it is an http page. `loadWith` takes the transport as
   a `fetcher`, which is why extraction is testable without a network.
3. **Extract.** `toMarkdown` (convert.go:143): `pickRoot` scores
   `<article>`/`<main>`/`<body>` by text weight, `dropTags` removes `<form>`
   and friends, `dropHeaders`/`dropChrome` strip the page chrome (with a 50%
   size guard so a chrome-shaped document is not emptied), code blocks keep
   their language (`language-*`, `data-language`, Sphinx `highlight-*`,
   MediaWiki `mw-highlight-lang-*`), tables are flattened, links are
   absolutized, then html-to-markdown v2 plus fence-aware cleanups
   (`tidyProse`, `mapOutsideFences`) and `sanitize` (control characters,
   including ESC, are removed — page content is untrusted).
4. **Render.** `renderDoc(md, width, cfg)` (render.go:133) parses the Markdown
   with goldmark, highlights code with chroma (memoized per block, one
   goroutine per block), and emits self-contained ANSI lines in a `Doc`:
   `Lines`, `Plain`, `Heads`, `Links` (as `LinkSpan` + `URLs`), per-line
   `Meta` for selection. Invisible APC markers placed during rendering are
   scanned into link spans afterwards, so links keep their screen positions.
5. **Interact.** The Bubble Tea v2 model (tui.go): keys, mouse, resize, spring
   scrolling (harmonica), search, selection, toasts, panels. Navigation
   (`browse.go`) keeps `back`/`fwd` stacks of `navEntry` and runs every load
   through `startNav`/`loadCmd`; an overtaken load is dropped by id, and a
   failed load never replaces the page on screen. `newModel` + `Update` +
   `render()` work with no terminal at all — that single property is what makes
   the deterministic UI tests and the agent sessions in `mcp_session.go`
   possible.

## Where things live

One `package main` at the root (the reader), plus the docs site and the
tooling. Nothing in the reader is exported and there is no `internal/`: the
server modes (`--mcp`, `--eval`) are files in the same package on purpose, so
they use `load`, `newModel` and the store directly without a public API to
maintain.

| Path | What |
|---|---|
| `main.go` | CLI entry: flags → config → `runTUI`, `--md`/pipe output, `--mcp`, `--eval` |
| `cli.go` | the flag table (`parseArgs` is driven by `flagDefs`) |
| `config.go` | `Config`, defaults, commented TOML load/save, `normalize` |
| `fetch.go` | `fetch`, `load`, `loadWith`, charset → UTF-8 |
| `convert.go` | media-type dispatch, HTML → Markdown, `sanitize`, `stripLinks` |
| `extract.go` | root picking, chrome dropping, table flattening, ESC escaping |
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

State is hand-rolled XDG (`XDG_STATE_HOME`), config and cache use Go's
`os.User*Dir` (which honor `XDG_CONFIG_HOME`/`XDG_CACHE_HOME` on Linux).
Directories are 0700. Every per-user path is read lazily per call, so tests
and benchmarks redirect all three with environment variables and nothing else
(`isolateAll` in tui_test.go, `benchIsolate` in bench_test.go).

## How everything is checked

| Surface | Covers | Run |
|---|---|---|
| Unit tests (`*_test.go`) | renderer + link positions, search, selection/copy, cache, config, keys, store, navigation (click, back/forward, failures, overtaken loads), extraction scenarios | `go test -race ./...` |
| QA corpora (`scripts/qa/`) | 460+ real URLs in 10 categories, chroma lexers, synthetic cases, PTY copy — output in `qa-out/` | `CORPUS=corpus python3 scripts/qa/harness.py` etc. |
| QA scenario tests (`qa_scenarios_test.go`) | every finding in `docs/QA-FINDINGS.md` has a test; a meta-test keeps the three in sync | `go test -race ./...` (`WR_QA_STRICT=1` asserts open findings) |
| `--eval` | 9 deterministic offline scenarios (input, extraction, render, links, session navigation, search, store, cache, config) as JSON, exit 1 on failure | `go run . --eval` |
| MCP smoke (`scripts/qa/mcp_smoke.py`) | the agent protocol end to end against any binary: handshake, tools/list, reading tools, a full session, config | `WR=./wr python3 scripts/qa/mcp_smoke.py` |
| Benchmarks (`bench_test.go`) | extraction, render, load path (cache vs no cache), interaction, store, search — local, deterministic, with allocations | `go test -run '^$' -bench . -benchmem` |

Two properties hold across all of them: **no network** (loopback `httptest`
stands in for one) and **no wall-clock dependence** (animations off, synthetic
pages). CI runs gofmt, `go mod tidy -diff`, `go vet`, `go test -race ./...`
and `go build ./...`; the git hooks repeat the fast ones on commit and push.

## The performance scenario

The measured baseline, the hot paths and the bottlenecks live in
[docs/BENCHMARKS.md](BENCHMARKS.md). The short version: extraction is the
slowest stage (4–5 MB/s, mostly the Markdown writer and the regexp cleanups,
not the HTML parse), rendering a large document costs ~25 ms and ~6.6 MB, the
cache turns a 21 ms miss into a 2.5 ms hit, and the interactive path is ~100 ns
per key with zero allocations. The budget every change should keep:

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
