# Performance: benchmarks, where the time goes

Everything hot in `wr` is measured locally and deterministically: the network
is a loopback `httptest` server, the fixtures are synthetic articles and
documents, animations are off, no wall-clock or external dependency. Run it
with the Go tooling you already have:

```sh
go test -run '^$' -bench . -benchmem        # the whole suite
go test -run '^$' -bench Render -benchmem   # one area
sh scripts/bench.sh                         # everything + profiles into bench-out/
```

The profiles answer "where does it get stuck":

```sh
go tool pprof -top -nodecount=20 bench-out/wr.test bench-out/cpu.prof
go tool pprof -top -sample_index=alloc_space bench-out/wr.test bench-out/mem.prof
```

The numbers below are the baseline: one run of `sh scripts/bench.sh` on a
Ryzen 5 3600 (6c/12t), Go 1.27.2, Linux, ext4. They are a comparison line, not
a promise: rerun on your machine before and after any change.

## The table

Sizes: a *medium* HTML article is ~90 KB with 12 code blocks; a *large* one is
~310 KB with 40. A *medium* Markdown document is ~26 KB (60 sections), a
*large* one ~230 KB (400 sections).

### Extraction (fetch → Markdown)

| Benchmark | Time | Throughput | Allocs |
|---|---|---|---|
| article small (~15 KB) | 3.6 ms | 4.3 MB/s | 4951 |
| article medium (~90 KB) | 22.0 ms | 4.2 MB/s | 27 K |
| article large (~310 KB) | 78.8 ms | 3.9 MB/s | 88 K |
| chrome-heavy (~10 KB, 40 navs) | 1.2 ms | 6.0 MB/s | 3544 |
| Markdown passthrough (~300 KB) | 4.4 ms | 20 MB/s | 137 |

### Load path (cache vs no cache)

| Benchmark | Time | Allocs |
|---|---|---|
| local file (~200 KB md) | 4.6 ms | 30 |
| loopback HTTP, cache off | 21.6 ms | 27 K |
| cache miss (fetch + write) | 21.4 ms | 27 K |
| **cache hit (disk, no network)** | **2.5 ms** | 16 |
| cache put+get (26 KB page) | 0.15 ms (347 MB/s) | 44 |

**Cache vs no cache is 8.5x on this path** (21.4 → 2.5 ms): the cache turns
"open instantly" into a disk read plus one `sanitize` pass over 26 KB, with 16
allocations. The hit path does no network, no parsing and no rendering of the
HTML — the Markdown was stored already extracted.

### Render (Markdown → ANSI)

| Benchmark | Time | Allocs |
|---|---|---|
| sample fixture, 80 cols | 0.24 ms | 887 |
| medium doc, 100 / 200 cols | 3.3 / 3.5 ms | 8.5 K |
| large doc, 100 / 200 / 40 cols | 25 / 25 / 29 ms | 51–84 K |
| code-heavy, 5 / 20 / 80 blocks | 1.8 / 7.0 / 28 ms | 8 K–162 K |
| re-render (chroma memoized) | 6.7 ms | 17 K |
| link mode: footnotes / inline / hidden | 2.9 / 3.0 / 2.6 ms | ~13 K |

### Interaction (what a keypress costs)

| Benchmark | Time | Allocs |
|---|---|---|
| scroll keys (j k d u G g) | ~100 ns | **0** |
| tab (link focus) | 354 ns | 2 |
| full screen draw (`render`) | 97 µs | 135 |
| resize (re-render at new width) | 8.0 ms | 19 K |
| in-page search (300 sections) | 53.8 ms | 7.7 K |
| follow a link (load + render) | 3.6 ms | 6.5 K |
| long document (1500 sections, load + 3 jumps) | 187 ms | 271 K |
| headless session open + close | 93.6 ms | 5.9 K |
| 8 keys through a session | 1.7 µs | 9 |

### Store and search

| Benchmark | Time | Allocs |
|---|---|---|
| history: 100 / 1000 / 5000 entries | 0.20 / 1.6 / 8.0 ms | 331–15 K |
| history append | 25 µs | 9 |
| bookmarks: 500 entries | 0.58 ms | 519 |
| in-page search over a 230 KB doc | 13.9 ms (31 MB/s) | 896 |

## Where it gets stuck

Three hot paths, in order of cost, with the evidence from the profiles.

**1. Extraction is the slowest thing wr does: 4–5 MB/s, and it is not the HTML
parse.** The parse alone is 4.4 ms of a 79 ms conversion (70 MB/s, ~6%). The
other ~94% is the Markdown writer and the cleanups around it: the heap profile
puts `renderDoc` and the regexp engine at the top (`regexp` is ~10% of all
samples: `tidyProse`, link stripping and the fence-aware helpers walk the whole
document several times). A large article pays ~79 ms of CPU before a single
line reaches the screen — the pipeline was tuned for correctness on hostile
pages, and each pass is a full scan.

**2. Rendering a large document costs 25 ms and 6.6 MB.** goldmark parses the
Markdown, then the custom renderer emits self-contained ANSI lines; 50–85 K
allocations per render is the price of building every line as a string.
`BenchmarkRenderReRender` shows the memoization working: highlighting is keyed
by the block, so the second render of the same document is 4x cheaper than a
code-heavy first one. Resizes and link-mode changes pay the full price again.

**3. In-page search over a big document is 54 ms, not the 14 ms of pure
matching.** `findMatches` alone runs at 31 MB/s (14 ms); the rest is
re-rendering the highlighted lines on every keystroke of the query and again on
`enter`. Typing "section 4" into a 300-section page means six full searches plus
six re-renders.

Smaller notes: `historyList()` reads and re-parses the whole JSONL every call
(8 ms at 5000 entries, and it is the top allocator), and the session layer
opens in ~94 ms — that is a load plus a first render, the keyboard path itself
is 100 ns per key with zero allocations.

## What is already fast

- The interactive path: 100 ns per key, no allocation, 97 µs per full screen.
  Scrolling and jumping never touch the document.
- The cache: 347 MB/s round trip, 8.5x fewer milliseconds than a fetch, and a
  hit costs 16 allocations.
- Markdown passthrough: 20 MB/s (the extractor only pays for HTML).

## The workflow: baseline, change, compare

The suite is built for before/after. `scripts/bench.sh` wraps it:

```sh
sh scripts/bench.sh baseline     # run everything, save it as bench-out/baseline.txt
# ...change something...
sh scripts/bench.sh compare     # run again and diff: exits 1 on a regression
```

`compare` prints a table of ns/op with the delta per benchmark plus allocation
deltas, and fails when any benchmark is worse than `BENCH_THRESHOLD_NS`
(default 25%; allocation growth over `BENCH_THRESHOLD_ALLOC`, default 40%, is a
warning). The verdict is machine-readable too:

```sh
python3 scripts/bench_compare.py --json bench-out/baseline.txt bench-out/compare-run.txt
```

Useful knobs: `BENCH_FILTER=Render sh scripts/bench.sh compare` (one area),
`BENCH_COUNT=3 sh scripts/bench.sh baseline` (repeat before trusting a delta
— the noise on a shared machine is real; re-run both sides when in doubt),
`BENCH_OUT=/tmp/b` to keep results out of the tree, and `sh scripts/bench.sh
profile Render` for just the profiles of an area.

The baseline itself carries its provenance (date, host, Go version) as comment
lines, so an old baseline says what it was measured against. Keep it in the
worktree out of git (`bench-out/` is ignored) — it is a local instrument, not
an artifact to share.

## Ideas the numbers suggest

None of these are implemented; they are what the profiles point at, in
descending order of expected win:

1. One combined cleanup pass over the extracted Markdown instead of the current
   per-rule scans (`mapOutsideFences` already avoids the fences; the same idea
   could carry `tidyProse`), which would attack the regexp-heavy 94%.
2. A cheaper highlight path for in-page search: only the matched lines need
   repainting, but today the whole screen is rebuilt.
3. Incremental history: keep an index (url → offset) so a 5000-entry history
   does not re-parse on every list.
4. Pool or reuse the line buffers in the renderer: 50–85 K allocations per
   large render is the GC pressure during resize.

## Methodology

- `b.Loop()`, one run (`-count 1`), 200 ms–1 s per benchmark, allocations
  always reported (`-benchmem`).
- Fixtures are synthetic but shaped like the real thing: prose sections,
  lists, tables, links, code fences in five languages, chrome to strip
  (`nav`, `share`, `footer`), and articles that grow past 300 KB.
- The network is loopback, so `LoadLoopback` measures fetch + convert +
  allocation, not the internet; a real page adds WAN latency on top.
- Cache benchmarks flip the real switch (`cacheOn`) and use the real
  `loadCmd`, not a stub of it.
