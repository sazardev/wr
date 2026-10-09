# QA harness

Reproducible tests behind the report in [`docs/QA.md`](../QA.md). Nothing here
is part of the build or runs in CI: these scripts hit the network, write their
output under `qa-out/` (ignored by git) and never touch your real `wr` config,
cache or history (all of it is redirected to an isolated `HOME`).

## Requirements

- Go (to build `wr` and the probes).
- `curl` and `python3` 3.10+.
- Linux for `copy_pty.py` (uses a PTY and SGR mouse sequences; no real terminal
  is needed).

```sh
go build -o wr .
export WR=$PWD/wr            # binary under test
export QA_ROOT=$(mktemp -d)  # raw outputs; default: ./qa-out
```

Environment:

| Variable | Default | Meaning |
|---|---|---|
| `WR` | `wr` from PATH | binary under test |
| `QA_ROOT` | `./qa-out` | outputs, isolated HOME, generated corpora |
| `WORKERS` | `6` | parallel fetchers in `harness.py` |
| `CORPUS` | `corpus` | corpus module to use (`corpus`, `corpus2`, `corpus3`) |
| `CHROMA_LEXERS` | `$QA_ROOT/chroma-lexers.tsv` | lexer inventory for `langs_corpus.py` |
| `CHROMA_TESTDATA` | auto (GOMODCACHE) | chroma `lexers/testdata` with official samples |

## Rounds

**1. Web reading.** For every URL: the original (`curl`), `wr --md` and the
piped render, with metrics of coverage, leaks, structure and timing.

```sh
CORPUS=corpus  python3 scripts/qa/harness.py   # 263 front pages/docs/feeds
CORPUS=corpus2 python3 scripts/qa/harness.py   # 113 harvested articles
CORPUS=corpus3 python3 scripts/qa/harness.py   # 86 language/framework docs
CORPUS=corpus4 python3 scripts/qa/harness.py   # 75 fresh URLs (round 2, 2026-10-09)
python3 scripts/qa/analyze.py                  # aggregates + worst cases
python3 scripts/qa/metrics.py                  # refined root-coverage metrics
```

`corpus4.py` exists so every round can add URLs that were never tested before:
keep the old corpora frozen (they are the baseline) and put the new batch in a
new `corpusN.py`.

**2. Code and languages.** One HTML page per lexer (297 chroma lexers + 11 `wr`
aliases, 262 with a real snippet), rendered and compared byte for byte.

```sh
go run ./scripts/qa/probe/lexers > "$QA_ROOT/chroma-lexers.tsv"
python3 scripts/qa/langs_corpus.py   # -> $QA_ROOT/lang-corpus.json (308)
python3 scripts/qa/langs_run.py      # fence detection, color, fidelity, label
```

**3. Synthetic cases.** Controlled HTML/Markdown for extraction, tables, i18n
and edge cases.

```sh
python3 scripts/qa/gen_synthetic.py
python3 scripts/qa/run_synthetic.py
```

**4. Copy.** Drives the real TUI in a PTY with SGR mouse drags and double/triple
clicks and decodes the OSC 52 clipboard payload.

```sh
python3 scripts/qa/copy_pty.py
```

## Go probes

| Probe | What it does |
|---|---|
| `probe/tomarkdown` | replicates `toMarkdown` with `dropTags`/`junkClass`/`codeJunk` word accounting per rule |
| `probe/lexers` | prints every chroma lexer (name, aliases, filenames) as TSV |
| `probe/hardwrap` | checks `ansi.Hardwrap` with the same call `render.go` makes |

```sh
go run ./scripts/qa/probe/tomarkdown page.html
```

## Notes

- Be polite: the default is 6 workers and one request per URL per capture.
- Failures from paywalls, bot walls and JavaScript-only sites are expected and
  classified by `analyze.py`; they are external to `wr`.
- The corpora are plain lists of URLs; edit or extend them freely.
