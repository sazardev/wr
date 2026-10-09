# QA plan: what to fix, in what order

Plan derived from the rounds documented in [`QA.md`](QA.md), with the ID register
in [`QA-FINDINGS.md`](QA-FINDINGS.md). Olas 1–4 are applied (see each finding's
*Resolución*), except QA-F-012, which is a product decision (see the register).
The automatic test mode (`qa_scenarios_test.go`) and the network harness
(`scripts/qa/`) are the verification surface.

## What the rounds covered

| Round | Input | Volume | Verdict |
|---|---|---|---|
| 1 | Front pages, docs, Wikipedia, news, papers, gov, forums, code hosts, odd formats, JS walls | 263 URLs | 232 extracted, 33% captured ≥60% |
| 1b | Real articles harvested from those pages | 113 URLs | 109 extracted, 64% captured ≥60% |
| 2 | Language rendering: 297 lexers + 11 aliases + 12 edge cases + clipboard | 308 + 12 + 4 | frames 320/320, color 304/308, code fidelity 88% |
| 2c | Language/framework documentation sites | 86 URLs | 81 extracted, 23% of fences carried a language |

## Guiding principles

1. **Never lose an article silently.** Ranking: losing the page (QA-F-001, -003)
   > wrong content (QA-F-002, -006) > no title (QA-F-004, -005) > noise
   (QA-F-012, -013).
2. **Code must be verbatim.** Fences are quoted data: post-processing must not
   rewrite them (QA-F-006, -007).
3. **Keep the security properties.** Control characters, `<form>` and header
   removals exist for good reasons; the fixes must narrow the blast radius, not
   delete the guard (QA-F-003, -004, -007).
4. **No guessing where we cannot know.** An unlabeled block is `text`; a wrong
   guess is worse than no color (QA-F-008).
5. **Every fix lands with a test.** `qa_scenarios_test.go` asserts the desired
   behavior; a fix flips the finding to `Resuelto` in the register and lets the
   corresponding subtest run for real.

## Work order

### Ola 1 — Cheap, high-impact extraction fixes (all S)

| ID | What to do | Risk to watch | Becomes green when |
|---|---|---|---|
| QA-F-010 | Decode UTF-8 first, fall back to windows-1252 in `fetch` (`main.go:270-276`) | none for valid UTF-8 | `TestQACharset/*` |
| QA-F-001 | Exact-token `junkClass` + "never remove >50% of the root text" guard (`main.go:51`, `336-340`) | `sidebar_junk_removed` regression | `TestQAExtraction/QA-F-001` |
| QA-F-003 | Drop form controls, not `<form>` (`main.go:46`) | stray labels in forms | `TestQAExtraction/QA-F-003` |
| QA-F-004 | Keep the first `<h1>` inside a removed `<header>` (`main.go:333-335`) | nav chrome leaking back | `TestQAExtraction/QA-F-004` |
| QA-F-005 | Title fallback must ignore fences (`main.go:389-391`) | none | `TestQAExtraction/QA-F-005` |

### Ola 2 — Code fidelity inside fences (QA-F-006, QA-F-007, M)

| ID | What to do | Risk to watch | Becomes green when |
|---|---|---|---|
| QA-F-006 | Make the `main.go:382-384` cleanup fence-aware (run it only outside fenced blocks) | prose gets slightly noisier | `TestQACodeFidelity/QA-F-006_*` |
| QA-F-007 | Let `ESC` survive inside code (render as `␛`, or scope the sanitizer to non-code text) | terminal injection surface | `TestQACodeFidelity/QA-F-007` |

### Ola 3 — Language detection and page noise (M)

| ID | What to do | Risk to watch | Becomes green when |
|---|---|---|---|
| QA-F-008 | Extend the class patterns and `langMap` (Sphinx, MediaWiki, common short names) | wrong guesses | `TestQALanguageDetection/QA-F-008_*` |
| QA-F-012 | Collapse duplicate URLs / cap the footnote list, or pick a smarter default | links must stay clickable | `TestQALinkModes/*` |
| QA-F-013 | Drop `role=navigation|banner|contentinfo` and exact-token junk classes | QA-F-001-style over-removal | `TestQAExtraction/QA-F-013` |

### Ola 4 — Structural: root choice and tables (M/L)

| ID | What to do | Risk to watch | Becomes green when |
|---|---|---|---|
| QA-F-002 | Score `article`/`main`/`body` candidates by text and paragraph density; keep `<article>` only when it wins | picking comments over content | `TestQAExtraction/QA-F-002` |
| QA-F-009 | Own the table conversion or pre-normalize cells (lists, spans) for the plugin | tables that work today | `TestQATables/QA-F-009` |

## Definition of done (per finding)

1. State in the register moves to `Resuelto` and the `Resuelto en` cell names the
   version/commit.
2. The subtest runs for real (it is not in `pendingFindings` anymore) and
   `go test ./...` is green.
3. `WR_QA_STRICT=1 go test -run TestQA ./...` is green (nothing pending fails).
4. Round 1–2 metrics do not regress: capture on real articles stays ≥60% median,
   code fidelity ≥95%, docs language rate improves against the 23% baseline.

## Verification: the automatic test mode

```sh
# offline, deterministic scenario suite (no network)
go test ./...                                   # normal: pending findings skip
WR_QA_STRICT=1 go test -run TestQA -v ./...     # assert open findings too (red until fixed)
```

The scenario suite is organized by document: `TestQAExtraction`,
`TestQACodeFidelity`, `TestQALanguageDetection`, `TestQATables`, `TestQACharset`,
`TestQAClipboard`, `TestQALinkModes`, and `TestQAFindingsCovered`, which fails
if a register ID has no scenario or a scenario has no register entry — that is
what keeps docs, register and tests in sync automatically.

Online round (needs network; the scripts are in `scripts/qa/`):

```sh
WR=$PWD/wr QA_ROOT=/tmp/qa python3 scripts/qa/harness.py           # CORPUS=corpus|corpus2|corpus3
WR=$PWD/wr QA_ROOT=/tmp/qa python3 scripts/qa/langs_run.py         # 308 languages
python3 scripts/qa/metrics.py                                      # coverage metrics
```

## Follow-up round (round 4)

After the olas land, rerun the same corpora and compare against this baseline:

| Metric | Round 2026-10-08 | Target |
|---|---|---|
| Real articles with ≥60% captured | 64% | ≥85% |
| Front pages with ≥60% captured | 33% | ≥55% |
| Fences with a language (docs) | 23% | ≥70% |
| Rendered code byte-identical | 88% | ≥99% |
| Boilerplate leaks (median) | 1 | 0 |

## Out of scope

Paywalls, JavaScript-only sites, sites that block non-browser clients and
unsupported formats: see the register's *External limits*. Their handling (a
notice, `x` to open, fallback to the previous page) is already correct.
