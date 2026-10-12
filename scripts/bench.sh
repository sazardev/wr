#!/bin/sh
# The performance toolkit for wr. Everything is local and deterministic: the
# network is a loopback server, no wall-clock dependencies, animation off.
#
#   sh scripts/bench.sh                      # the whole suite + cpu/heap profiles
#   sh scripts/bench.sh baseline             # save the current numbers as the baseline
#   sh scripts/bench.sh compare              # run and diff against the baseline (exit 1 on regressions)
#   sh scripts/bench.sh profile [filter]     # profiles of one area (default: everything)
#
# Environment:
#   BENCH_FILTER=.        a regexp of benchmarks (e.g. Render, Cache)
#   BENCH_COUNT=1         repetitions per benchmark (3+ for variance)
#   BENCH_THRESHOLD_NS=25 % worse in ns/op that fails compare
#   BENCH_OUT=bench-out   where results and profiles land (gitignored)
set -eu

cd "$(git rev-parse --show-toplevel)"

OUT=${BENCH_OUT:-bench-out}
FILTER=${BENCH_FILTER:-.}
COUNT=${BENCH_COUNT:-1}
THRESHOLD=${BENCH_THRESHOLD_NS:-25}
BASELINE="$OUT/baseline.txt"

mkdir -p "$OUT"

run_suite() { # run_suite <output.txt> [extra go test flags...]
    _out=$1
    shift
    echo "== go vet ==" >&2
    go vet ./...
    echo "== benchmarks (filter: $FILTER, count: $COUNT) ==" >&2
    # shellcheck disable=SC2086
    go test -run '^$' -bench "$FILTER" -benchmem -count "$COUNT" "$@" . | tee "$_out"
}

cmd=${1:-all}

case "$cmd" in
all)
    run_suite "$OUT/bench.txt" -cpuprofile "$OUT/cpu.prof" -memprofile "$OUT/mem.prof" -o "$OUT/wr.test"
    echo
    echo "== where the time goes (cpu) =="
    go tool pprof -top -nodecount=20 "$OUT/wr.test" "$OUT/cpu.prof" | tee "$OUT/cpu-top.txt" || true
    echo
    echo "== where the allocations go (heap) =="
    go tool pprof -top -sample_index=alloc_space -nodecount=20 "$OUT/wr.test" "$OUT/mem.prof" | tee "$OUT/mem-top.txt" || true
    echo
    echo "results in $OUT/bench.txt, profiles in $OUT/"
    ;;

baseline)
    {
        echo "# wr performance baseline"
        echo "# saved: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
        echo "# host: $(uname -srm)"
        echo "# go: $(go version | awk '{print $3}')"
    } > "$BASELINE"
    run_suite "$BASELINE.tmp" >> "$BASELINE"
    mv "$BASELINE.tmp" "$BASELINE"
    echo
    echo "baseline saved in $BASELINE ($(grep -c '^Benchmark' "$BASELINE") benchmarks)"
    ;;

compare)
    if [ ! -f "$BASELINE" ]; then
        echo "no baseline yet: run 'sh scripts/bench.sh baseline' first" >&2
        exit 2
    fi
    run_suite "$OUT/compare-run.txt" -o "$OUT/wr.test"
    echo
    echo "== against $BASELINE =="
    if BENCH_THRESHOLD_NS="$THRESHOLD" python3 scripts/bench_compare.py "$BASELINE" "$OUT/compare-run.txt"; then
        echo
        echo "no regressions over ${THRESHOLD}% (run kept in $OUT/compare-run.txt)"
    else
        _rc=$?
        echo
        echo "regressions over ${THRESHOLD}% in $OUT/compare-run.txt" >&2
        exit "$_rc"
    fi
    ;;

profile)
    shift
    run_suite "$OUT/bench.txt" -cpuprofile "$OUT/cpu.prof" -memprofile "$OUT/mem.prof" -o "$OUT/wr.test"
    echo
    echo "== where the time goes (cpu) =="
    go tool pprof -top -nodecount=25 "$OUT/wr.test" "$OUT/cpu.prof" | tee "$OUT/cpu-top.txt" || true
    echo
    go tool pprof -top -sample_index=alloc_space -nodecount=25 "$OUT/wr.test" "$OUT/mem.prof" | tee "$OUT/mem-top.txt" || true
    echo
    echo "For a deeper look: go tool pprof -http=:8080 $OUT/wr.test $OUT/cpu.prof"
    ;;

*)
    echo "usage: sh scripts/bench.sh [all|baseline|compare|profile [filter]]" >&2
    exit 2
    ;;
esac
