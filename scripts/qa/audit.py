#!/usr/bin/env python3
"""Audit wr's extraction against real pages (QA round 3).

Runs `wr --md` over a list of URLs and flags what reads badly: backslash line
breaks, empty links, repeated lines, thin results, JS-shell notices, errors, and
prints the first lines of every page so navigation junk at the top is easy to
spot. It touches the network but never your real wr config, cache or history
(everything is redirected to a temporary HOME).

    WR=./wr python3 scripts/qa/audit.py                  # the default list
    WR=./wr python3 scripts/qa/audit.py --urls my.txt --jobs 4 --out /tmp/audit

Compare two builds by running it twice with different --out directories and
`diff -r`-ing the .md files; the summary shows word counts per page.
"""
import argparse
import concurrent.futures as cf
import os
import re
import subprocess
import sys
import tempfile
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent


def run_one(wr, env, out, i, url, timeout):
    start = time.time()
    try:
        p = subprocess.run([wr, "--md", url], capture_output=True, text=True, timeout=timeout, env=env)
        md, err, rc = p.stdout, p.stderr.strip(), p.returncode
    except subprocess.TimeoutExpired:
        md, err, rc = "", "TIMEOUT", 124
    (out / f"{i:02d}.md").write_text(md)
    return {"i": i, "url": url, "md": md, "err": err, "rc": rc, "secs": time.time() - start}


def anomalies(md):
    # code fences are quoted data: only judge the prose
    lines, fence = [], None
    for l in md.split("\n"):
        m = re.match(r"^\s*(`{3,}|~{3,})", l)
        if fence:
            if m and m.group(1)[0] == fence[0] and len(m.group(1)) >= len(fence) and l.strip() == m.group(1):
                fence = None
            continue
        if m:
            fence = m.group(1)
            continue
        lines.append(l)
    prose = "\n".join(lines)
    nz = [l.strip() for l in lines if l.strip() and not l.strip().startswith("|")]
    found = {}
    n = sum(1 for l in lines if l.strip() == "\\" or l.rstrip().endswith("\\"))
    if n:
        found["backslash-breaks"] = n
    n = sum(1 for a, b in zip(nz, nz[1:]) if a == b and len(a) > 12)
    if n:
        found["repeated-lines"] = n
    n = len(re.findall(r"\[\s*\]\(", prose))
    if n:
        found["empty-links"] = n
    n = len(re.findall(r"(?i)skip to (the )?(main )?content", prose[:600]))
    if n:
        found["skip-link"] = n
    return found, nz


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--urls", default=str(HERE / "audit_urls.txt"))
    ap.add_argument("--out", default=None, help="directory for the .md outputs (default: a temp dir)")
    ap.add_argument("--jobs", type=int, default=8)
    ap.add_argument("--timeout", type=int, default=100)
    ap.add_argument("--tops", action="store_true", help="print the first lines of every page")
    args = ap.parse_args()

    wr = os.environ.get("WR", "wr")
    urls = [u.strip() for u in Path(args.urls).read_text().splitlines() if u.strip() and not u.startswith("#")]
    out = Path(args.out or tempfile.mkdtemp(prefix="wr-audit-"))
    out.mkdir(parents=True, exist_ok=True)

    home = tempfile.mkdtemp(prefix="wr-audit-home-")
    env = dict(os.environ, HOME=home, XDG_CONFIG_HOME=home + "/c", XDG_STATE_HOME=home + "/s",
               XDG_CACHE_HOME=home + "/k", WR_NO_UPDATE_CHECK="1")

    with cf.ThreadPoolExecutor(args.jobs) as ex:
        results = list(ex.map(lambda t: run_one(wr, env, out, t[0], t[1], args.timeout), enumerate(urls)))

    print(f"# {len(results)} pages -> {out}\n")
    print("== errors, thin pages, JS shells")
    for r in results:
        words = len(r["md"].split())
        shell = "draws its content with JavaScript" in r["md"]
        if r["rc"] or r["err"] or words < 150 or shell:
            note = r["err"][:100] or ("SHELL" if shell else "")
            print(f"{r['i']:02d} {words:6d}w {r['secs']:5.1f}s rc={r['rc']} {r['url']}  {note}")
    print("\n== anomalies")
    tops = []
    for r in results:
        found, nz = anomalies(r["md"])
        if found:
            print(f"{r['i']:02d} {found} {r['url']}")
        tops.append((r["i"], r["url"], nz[:3]))
    print("\n== slow (>12s)")
    for r in results:
        if r["secs"] > 12:
            print(f"{r['i']:02d} {r['secs']:.1f}s {r['url']}")
    if args.tops:
        print("\n== first lines")
        for i, url, nz in tops:
            print(f"{i:02d} {url[8:52]:44s} | " + " ¦ ".join(l[:42] for l in nz))
    print("\nWords per page:")
    print(" ".join(f"{r['i']:02d}:{len(r['md'].split())}" for r in results))


if __name__ == "__main__":
    sys.exit(main())
