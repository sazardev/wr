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

The numbers below are the current baseline: `scripts/bench.sh baseline`
(count=3, medianas) on a Ryzen 5 3600 (6c/12t), Go 1.27.2, Linux, ext4. They
are a comparison line, not a promise: rerun on your machine before and after
any change, and read "The workflow" at the end about what counts as a
regression on a shared machine.

## The table

Sizes: a *medium* HTML article is ~90 KB with 12 code blocks; a *large* one is
~310 KB with 40. A *medium* Markdown document is ~26 KB (60 sections), a
*large* one ~230 KB (400 sections). *(nuevo)* marks what changed in the
optimization pass of 2026-10-11; the deltas are measured with the old and the
new code alternating on the same session, which the single-run noise of this
machine demands.

### Extraction (fetch → Markdown) *(nuevo: -12 a -37%)*

| Benchmark | Before | After | Δ |
|---|---|---|---|
| article small (~15 KB) | 5.3 ms | 4.7 ms | -12% |
| article medium (~90 KB) | 25.7 ms | 16.1 ms | **-37%** |
| article large (~310 KB) | — | ~40 ms (count=3) | **-30%** |
| chrome-heavy (~10 KB, 40 navs) | 1.2 ms | ~1.1 ms | ~-10% |
| Markdown passthrough (~300 KB) | 4.4 ms | 4.5 ms | ~0% |

### Load path (cache vs no cache) *(sin cambio)*

| Benchmark | Time | Allocs |
|---|---|---|
| local file (~200 KB md) | 3.9 ms | 31 |
| loopback HTTP, cache off | 15.5 ms | 26 K |
| cache miss (fetch + write) | 15.8 ms | 27 K |
| **cache hit (disk, no network)** | **2.1 ms** | 17 |
| cache put+get (26 KB page) | 0.25 ms (347 MB/s) | 44 |

**Cache vs no cache is ~7.5x on this path** (15.8 → 2.1 ms): the cache turns
"open instantly" into a disk read plus one `sanitize` pass over 26 KB, with 17
allocations. The hit path does no network, no parsing and no rendering of the
HTML — the Markdown was stored already extracted.

### Render (Markdown → ANSI) *(nuevo: -15%)*

| Benchmark | Before | After | Δ |
|---|---|---|---|
| sample fixture, 80 cols | 0.24 ms | 0.18 ms | ~-20% |
| medium doc, 100 cols | 2.69 ms | 2.28 ms | **-15%** |
| large doc, 100 cols | 20.8 ms | 17.5 ms | **-16%** |
| large doc, 200 cols | 25.0 ms | ~19 ms | ~-24% |
| code-heavy, 5 / 20 / 80 blocks | 1.8 / 7.0 / 28 ms | 1.5 / 6.4 / 22 ms | -15/-9/-22% |
| re-render (chroma memoized) | 6.7 ms | ~6.5 ms | ~-3% |
| link mode: footnotes / inline / hidden | 2.9 / 3.0 / 2.6 ms | ~2.1-2.3 / 2.2-2.3 / 1.9 | ~-25% |

### Interaction (what a keypress costs) *(nuevo: search -65%)*

| Benchmark | Before | After | Δ |
|---|---|---|---|
| scroll keys (j k d u G g) | ~100 ns | ~100 ns | 0 |
| tab (link focus) | 354 ns | ~330 ns | ~0 |
| full screen draw (`render`) | 97 µs | ~80 µs | ~-15% |
| resize (re-render at new width) | 8.0 ms | ~5 ms | ~-30% |
| **in-page search (300 sections)** | **53.8 ms, 7.7 K allocs** | **~31 ms, 99 allocs** | **-42%, -99% allocs** |
| follow a link (load + render) | 3.6 ms | ~3.5 ms | ~0 |
| long document (1500 sections, load + 3 jumps) | 187 ms | ~160 ms | ~-15% |
| headless session open + close | 93.6 ms | ~93 ms | ~0 |
| 8 keys through a session | 1.7 µs | ~1.7 µs | 0 |

### Store and search *(nuevo: findMatches -48%)*

| Benchmark | Before | After | Δ |
|---|---|---|---|
| **`findMatches` (230 KB doc)** | **20.9 ms, 896 allocs** | **10.8 ms, 10 allocs** | **-48%, -99% allocs** |
| history: 100 / 1000 / 5000 entries | 0.20 / 1.6 / 8.0 ms | ~0.17 / 1.4 / 7.5 ms | ~-5% |
| history append | 25 µs | ~40 µs | *noise* |
| bookmarks: 500 entries | 0.58 ms | ~0.49 ms | ~-15% |

The interactive path stays what it was: ~100 ns per key with zero
allocations, and a cached page open stays 2 ms. Those were never the problem.

## Where it got stuck, and what changed

The optimization pass of 2026-10-11 attacked the three hot paths the profiles
had found, in order of cost. Every change keeps the output byte-identical: the
unit tests, `wr --eval` (107 checks) and the MCP smoke verify it.

**1. Extraction — the cleanup, not the parse (medium: -37%).** The HTML parse
alone was 6% of a large conversion; the rest was the Markdown writer plus our
own cleanups. `tidyMarkdown` (extract.go) now does the whole prose cleanup in
one hand-rolled pass per line — fence detection by hand (the fence regexp ran
once per line of prose and was the single most expensive regexp of the
extraction), link-only line detection by scan, trailing-whitespace trim, blank
run collapse — replacing three `ReplaceAllString` passes per stretch plus the
per-line fence regexp. `hasTitleHeading` walks the same way instead of two
regexp passes, and `sanitize` (a full-document regexp with a fast no-match
path) stays for what comes back from the cache.

**2. In-page search — no regexp engine (-48% the matcher, -65% the whole
search, allocations x100 down).** The query is always a literal (it is quoted
for the engine), so `findMatches` (search.go) now matches it by hand over a
64-rune window with `unicode.ToLower` folding, in one pass per line, with a
window reset after each match so matches do not overlap like
`FindAllStringIndex`. The regexp path stays for absurd queries (> 64 runes).
The search benchmark now spends its time in the re-render of the highlighted
lines instead of the engine: 7675 → 99 allocations per keystroke.

**3. Render — Plain without the ANSI parser (-15/16%).** `plain[i]` was built
with `ansi.Strip`, a full escape-sequence parser, and it cost a sixth of a
large-document render on lines that wr itself painted with SGR and APC markers
(the markers are already stripped by then). `stripANSI` (render.go) walks
CSI sequences and OSC/DCS strings directly, and returns the line untouched
when it has no ESC at all.

## What the profiles no longer show

Before, the top of a render was `ansi.wrap`, `bytes.Buffer.WriteByte` and
`ansi.Strip`; after, the render is magnetized by the line building
(`r.block`/`selfContain`) and the wrap itself, and extraction is dominated by
the html-to-markdown library instead of our own regexps. What is left, in
descending order of expected win:

1. **Pool or reuse the line buffers in the renderer**: 50 K allocations per
   large render is the GC pressure during resize.
2. **A cheaper highlight path for in-page search**: only the matched lines
   need repainting, but today the whole screen is rebuilt per keystroke (this
   is now the dominant cost of a search, ~31 ms).
3. **Incremental history**: keep an index (url → offset) so a 5000-entry
   history does not re-parse on every list.

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

**Noise on a shared machine is real** — single runs of the sub-millisecond
benchmarks swing ±30%. Use `BENCH_COUNT=3` (or more) before trusting any delta
under 25%, keep the filter tight while iterating
(`BENCH_FILTER=Render scripts/bench.sh compare`), and when a change matters,
alternate the two code versions in the same session (a `git stash` of the four
hot files, then swap) as this pass did. `BENCH_OUT=/tmp/b` keeps results out
of the tree.

The baseline itself carries its provenance (date, host, Go version) as comment
lines, so an old baseline says what it was measured against. Keep it in the
worktree out of git (`bench-out/` is ignored) — it is a local instrument, not
an artifact to share.

## Methodology

- `b.Loop()`, count=1 by default and 3+ for the numbers quoted here, 200 ms–1 s
  per benchmark, allocations always reported (`-benchmem`).
- Fixtures are synthetic but shaped like the real thing: prose sections,
  lists, tables, links, code fences in five languages, chrome to strip
  (`nav`, `share`, `footer`), and articles that grow past 300 KB.
- The network is loopback, so `LoadLoopback` measures fetch + convert +
  allocation, not the internet; a real page adds WAN latency on top.
- Cache benchmarks flip the real switch (`cacheOn`) and use the real
  `loadCmd`, not a stub of it.
