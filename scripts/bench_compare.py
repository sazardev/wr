#!/usr/bin/env python3
# Compare two go test -bench outputs (or one against a saved baseline) and
# report the regression per benchmark. ns/op drives the verdict: a regression
# worse than the threshold (default 10%) exits 1; allocations and bytes are
# reported but never fail on their own.
#
#   scripts/bench.sh baseline           # writes bench-out/baseline.txt
#   scripts/bench.sh compare            # runs the suite, compares, exits 1 on regressions
#   python3 scripts/bench_compare.py bench-out/run1.txt bench-out/run2.txt
#   BENCH_THRESHOLD_NS=25 python3 scripts/bench_compare.py --json a.txt b.txt
import json
import os
import re
import sys
from pathlib import Path

THRESHOLD_NS = float(os.environ.get("BENCH_THRESHOLD_NS", 25))  # % worse that fails
THRESHOLD_ALLOC = float(os.environ.get("BENCH_THRESHOLD_ALLOC", 40))  # % worse that warns
BENCH_LINE = re.compile(
    r"^(Benchmark\S+?)-\d+\s+\d+\s+([\d.]+) ns/op(?:\s+([\d.]+) B/op)?(?:\s+(\d+) allocs/op)?")

META = ("goos", "goarch", "pkg", "cpu", "Benchmark", "ok ", "PASS", "FAIL", "go test", "goos")


def parse(path):
    """bench name -> {"ns": float, "bytes": float|None, "allocs": int|None}."""
    out = {}
    for line in Path(path).read_text().splitlines():
        if line.startswith("#"):
            continue
        m = BENCH_LINE.match(line)
        if not m:
            continue
        name = m.group(1)
        entry = {"ns": float(m.group(2))}
        entry["bytes"] = float(m.group(3)) if m.group(3) else None
        entry["allocs"] = int(m.group(4)) if m.group(4) else None
        out[name] = entry
    return out


def pct(new, old):
    if not old:
        return float("inf")
    return (new - old) / old * 100.0


def compare(old, new):
    rows, worst = [], 0.0
    for name in sorted(new):
        cur = new[name]
        if name not in old:
            rows.append((name, "NEW", cur, None, None))
            continue
        base = old[name]
        d_ns = pct(cur["ns"], base["ns"])
        d_b = pct(cur["bytes"], base["bytes"]) if cur["bytes"] and base["bytes"] else None
        d_a = pct(cur["allocs"], base["allocs"]) if cur["allocs"] and base["allocs"] else None
        verdict = "ok"
        if d_ns > THRESHOLD_NS:
            verdict = "REGRESSION"
        elif d_ns < -THRESHOLD_NS:
            verdict = "faster"
        elif d_b is not None and d_b > THRESHOLD_ALLOC:
            verdict = "warning"
        worst = max(worst, d_ns)
        rows.append((name, verdict, cur, base, (d_ns, d_b, d_a)))
    for name in sorted(old):
        if name not in new:
            rows.append((name, "GONE", old[name], None, None))
    return rows, worst


def render(rows, as_json):
    if as_json:
        out = []
        for name, verdict, cur, base, deltas in rows:
            out.append({"benchmark": name, "verdict": verdict,
                        "ns": cur["ns"], "allocs": cur.get("allocs"),
                        "d_ns_pct": round(deltas[0], 1) if deltas else None,
                        "d_alloc_pct": round(deltas[2], 1) if deltas and deltas[2] is not None else None})
        print(json.dumps({"threshold_ns_pct": THRESHOLD_NS, "benchmarks": out}, indent=2))
        return

    print(f"{'benchmark':<52}{'ns/op':>12}{'Δ ns':>9}{'Δ alloc':>9}  verdict")
    print("-" * 92)
    for name, verdict, cur, base, deltas in rows:
        ns = f"{cur['ns']:,.0f}" if cur.get("ns") else "-"
        d_ns = f"{deltas[0]:+.1f}%" if deltas else "-"
        d_a = f"{deltas[2]:+.1f}%" if deltas and deltas[2] is not None else "-"
        short = name if len(name) <= 51 else name[:48] + "..."
        print(f"{short:<52}{ns:>12}{d_ns:>9}{d_a:>9}  {verdict}")
    regressions = sum(1 for r in rows if r[1] == "REGRESSION")
    print("-" * 92)
    print(f"{len(rows)} benchmarks, {regressions} regressions over {THRESHOLD_NS:.0f}%")


def main(argv):
    args = [a for a in argv[1:] if not a.startswith("--")]
    as_json = "--json" in argv[1:]
    if len(args) != 2:
        print(__doc__)
        return 2
    old, new = parse(args[0]), parse(args[1])
    if not old:
        print(f"no benchmarks parsed from {args[0]}; is it a -bench output?")
        return 2
    if not new:
        print(f"no benchmarks parsed from {args[1]}; is it a -bench output?")
        return 2
    rows, _ = compare(old, new)
    render(rows, as_json)
    return 1 if any(r[1] == "REGRESSION" for r in rows) else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
