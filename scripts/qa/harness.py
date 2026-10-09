#!/usr/bin/env python3
# QA harness: for every URL it downloads the original (curl), extracts the
# Markdown (wr --md) and renders the real pipe view (wr page.md). Raw outputs
# and metrics are written next to the results (results.json / summary.csv).
import concurrent.futures as cf
import json
import os
import re
import subprocess
import sys
import time
from html import unescape
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
import importlib
CORPUS = importlib.import_module(os.environ.get("CORPUS", "corpus")).CORPUS  # noqa: E402

ROOT = Path(os.environ.get("QA_ROOT", "qa-out")).resolve()
PAGES = ROOT / "pages"
WR = os.environ.get("WR", "wr")
HOME = ROOT / "home"
HOME.mkdir(parents=True, exist_ok=True)
UA = "Mozilla/5.0 (X11; Linux x86_64) wr/2.0"
env = dict(os.environ)
env.update(
    HOME=str(HOME),
    XDG_CONFIG_HOME=str(HOME / ".config"),
    XDG_CACHE_HOME=str(HOME / ".cache"),
    XDG_STATE_HOME=str(HOME / ".local" / "state"),
)
env.pop("COLUMNS", None)

ANSI_RE = re.compile(r"\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])")
WORD_RE = re.compile(r"[^\W_]+", re.UNICODE)

LEAKS = {
    "cookie/consent": r"(?i)\b(cookies?|consentimiento|consent|aceptar todas|accept all)\b",
    "newsletter": r"(?i)\b(newsletter|bolet[ií]n|suscr[ií]bete|subscribe to)\b",
    "signin/login": r"(?i)\b(sign in|log in|iniciar sesi[oó]n|acceder|register|reg[ií]strate)\b",
    "ads": r"(?i)\b(advertisement|publicidad|sponsored|patrocinado)\b",
    "share": r"(?i)\b(share this|comparte|compartir|copy link|comparte este art[ií]culo)\b",
    "related": r"(?i)\b(related posts?|art[ií]culos relacionados|te puede interesar|read more|leer m[aá]s)\b",
    "comments": r"(?i)\b(comments?|comentarios)\b",
    "footer/legal": r"(?i)(all rights reserved|todos los derechos reservados|privacy policy|pol[ií]tica de privacidad|terms of (use|service))",
    "social/follow": r"(?i)\b(follow us|s[ií]guenos|twitter|facebook|instagram|linkedin|youtube|telegram|whatsapp)\b",
    "nav/menu": r"(?i)\b(skip to (main )?content|navigation|men[uú] principal|main menu|breadcrumb)\b",
    "app-push": r"(?i)\b(download (our|the) app|descarga (nuestra|la) app|enable javascript|activa javascript)\b",
    "paywall": r"(?i)\b(subscribe now|suscr[ií]bete ahora|already a subscriber|ya eres suscriptor|paywall|premium)\b",
}

HTML_ENTITIES = [("&nbsp;", " "), ("&amp;", "&"), ("&lt;", "<"), ("&gt;", ">")]


def strip_html(text: str) -> str:
    text = re.sub(r"(?is)<(script|style|noscript|svg|template)[^>]*>.*?</\1>", " ", text)
    text = re.sub(r"(?s)<!--.*?-->", " ", text)
    text = re.sub(r"(?s)<[^>]+>", " ", text)
    return re.sub(r"\s+", " ", unescape(text)).strip()


def strip_ansi(b: bytes) -> str:
    return ANSI_RE.sub("", b.decode("utf-8", "replace"))


def words(text: str):
    return [w.casefold() for w in WORD_RE.findall(text) if len(w) > 1 or w.isalnum()]


def sentences(text: str, n=40):
    parts = re.split(r"(?<=[.!?])\s+", text)
    out = [p for p in parts if len(p) >= 50]
    if len(out) <= n:
        return out
    step = len(out) / n
    return [out[int(i * step)] for i in range(n)]


def run(cmd, timeout, **kw):
    t0 = time.time()
    try:
        p = subprocess.run(cmd, capture_output=True, timeout=timeout, **kw)
        return p.returncode, p.stdout, p.stderr, time.time() - t0, False
    except subprocess.TimeoutExpired as e:
        return 124, e.stdout or b"", e.stderr or b"", time.time() - t0, True


def fetch_original(url, out):
    cmd = [
        "curl", "-sSL", "--compressed", "--max-time", "25", "-A", UA,
        "-o", str(out), "-w", "%{http_code}|%{content_type}|%{size_download}|%{time_total}|%{url_effective}",
        url,
    ]
    code, o, e, dt, to = run(cmd, 45)
    meta = {"curl_rc": code, "timeout": to, "time": dt, "stderr": e.decode("utf-8", "replace")[-400:]}
    if code == 0:
        parts = o.decode("utf-8", "replace").strip().split("|")
        if len(parts) == 5:
            meta.update(http_code=parts[0], content_type=parts[1], size=int(float(parts[2])), http_time=float(parts[3]), final_url=parts[4])
    return meta


def fetch_wr(url, mdpath):
    cmd = ["timeout", "-k", "2", "45", WR, "--md", url]
    code, o, e, dt, to = run(cmd, 60, env=env)
    if code == 0 and o:
        mdpath.write_bytes(o)
        return {"wr_rc": 0, "time": dt, "timeout": to, "stderr": e.decode("utf-8", "replace").strip()[-300:]}
    # retry once on timeout / transient
    if code != 0:
        time.sleep(3)
        code, o, e, dt, to = run(cmd, 60, env=env)
        if code == 0 and o:
            mdpath.write_bytes(o)
    return {"wr_rc": code, "time": dt, "timeout": to, "stderr": e.decode("utf-8", "replace").strip()[-300:]}


def fetch_render(mdpath, rpath, width=100):
    cmd = ["timeout", "-k", "2", "30", WR, str(mdpath)]
    code, o, e, dt, to = run(cmd, 40, env=dict(env, COLUMNS=str(width)))
    if code == 0:
        rpath.write_bytes(o)
    return {"render_rc": code, "render_time": dt, "render_timeout": to, "render_stderr": e.decode("utf-8", "replace").strip()[-300:]}


def metrics(url, cat, orig_path, md_path, render_path, meta):
    m = {"i": meta["i"], "url": url, "category": cat}
    m.update(meta)
    m["original_bytes"] = orig_path.stat().st_size if orig_path.exists() else 0
    orig_html = orig_path.read_text("utf-8", "replace") if orig_path.exists() else ""
    orig_text = strip_html(orig_html)
    m["orig_words"] = len(words(orig_text))
    # counts in original HTML
    def cnt(pat, flags=re.I):
        return len(re.findall(pat, orig_html, flags))
    m["o_h1"] = cnt(r"<h1\b")
    m["o_h2"] = cnt(r"<h2\b")
    m["o_pre"] = cnt(r"<pre\b")
    m["o_table"] = cnt(r"<table\b")
    m["o_img"] = cnt(r"<img\b")
    m["o_a"] = cnt(r"<a\b")
    m["o_code"] = cnt(r"<code\b")
    m["o_ul"] = cnt(r"<ul\b")
    m["o_blockquote"] = cnt(r"<blockquote\b")

    md = md_path.read_text("utf-8", "replace") if md_path.exists() else ""
    m["md_bytes"] = len(md.encode())
    m["md_lines"] = md.count("\n")
    m["md_words"] = len(words(md))
    m["md_has_title"] = bool(re.match(r"\s*#\s+\S", md))
    m["md_headings"] = len(re.findall(r"(?m)^#{1,6}\s", md))
    m["md_fences"] = len(re.findall(r"(?m)^(`{3,}|~{3,})", md)) // 2
    m["md_fence_langs"] = re.findall(r"(?m)^`{3,}([A-Za-z0-9+#._-]+)", md)
    m["md_tables"] = len(re.findall(r"(?m)^\|.*\|\s*$", md))
    m["md_images"] = len(re.findall(r"!?\[image:", md))
    m["md_links"] = len(re.findall(r"(?<!!)\[[^\]]*\]\([^)]*\)", md))
    m["md_bare_links"] = len(re.findall(r"(?m)^\s*(?:\[[^\]]*\]\([^)]*\)\s*)+$", md))
    m["md_lists"] = len(re.findall(r"(?m)^\s*(?:[-*+]|\d+\.)\s", md))

    # leaks in extracted markdown
    leaks = {}
    for name, pat in LEAKS.items():
        c = len(re.findall(pat, md))
        if c:
            leaks[name] = c
    m["leaks"] = leaks

    # content coverage: sampled sentences of the original found in the markdown
    norm_md = re.sub(r"\s+", " ", md).casefold()
    sents = sentences(orig_text)
    if sents:
        found = 0
        for s in sents:
            probe = re.sub(r"\s+", " ", s).casefold()[:60]
            if len(probe) >= 30 and probe in norm_md:
                found += 1
        m["sentence_recall"] = round(found / len(sents), 3)
        m["sentence_sample"] = len(sents)
    else:
        m["sentence_recall"] = None

    # word overlap
    ow, mw = set(words(orig_text)), set(words(md))
    m["word_recall"] = round(len(ow & mw) / max(1, len(ow)), 3)
    m["word_precision"] = round(len(ow & mw) / max(1, len(mw)), 3)
    m["md_vs_orig_ratio"] = round(m["md_words"] / max(1, m["orig_words"]), 3)

    # rendered view
    if render_path.exists():
        rend_raw = render_path.read_bytes()
        rend = strip_ansi(rend_raw)
        m["render_bytes"] = len(rend_raw)
        m["render_lines"] = rend.count("\n")
        m["render_words"] = len(words(rend))
        m["render_ansi_seqs"] = len(ANSI_RE.findall(rend_raw.decode("utf-8", "replace")))
        m["render_leftover_heading"] = len(re.findall(r"(?m)^#{1,6}\s", rend))
        m["render_leftover_fence"] = len(re.findall(r"(?m)^(`{3,}|~{3,})", rend))
        m["render_raw_link_syntax"] = len(re.findall(r"\]\(https?://", rend))
        m["render_word_recall"] = round(len(set(words(rend)) & ow) / max(1, len(ow)), 3) if ow else None
    m["leak_total"] = sum(leaks.values())
    return m


def process(item):
    i, (cat, url) = item
    d = PAGES / f"{i:03d}"
    d.mkdir(parents=True, exist_ok=True)
    (d / "url.txt").write_text(url)
    orig_path = d / "orig.html"
    md_path = d / "wr.md"
    render_path = d / "render.txt"
    meta_path = d / "meta.json"
    if meta_path.exists() and not os.environ.get("FORCE"):
        meta = json.loads(meta_path.read_text())
    else:
        meta = {"i": i, "category": cat}
        meta.update(fetch_original(url, orig_path))
        meta.update(fetch_wr(url, md_path))
        if meta.get("wr_rc") == 0 and md_path.exists() and md_path.stat().st_size > 0:
            meta.update(fetch_render(md_path, render_path))
        meta_path.write_text(json.dumps(meta, indent=1, ensure_ascii=False))
    try:
        return metrics(url, cat, orig_path, md_path, render_path, meta)
    except Exception as e:  # noqa: BLE001
        return {"i": i, "url": url, "category": cat, "harness_error": repr(e)}


def main():
    PAGES.mkdir(parents=True, exist_ok=True)
    items = list(enumerate(CORPUS))
    workers = int(os.environ.get("WORKERS", "6"))
    results = [None] * len(items)
    t0 = time.time()
    with cf.ThreadPoolExecutor(max_workers=workers) as ex:
        futs = {ex.submit(process, it): it for it in items}
        for k, fut in enumerate(cf.as_completed(futs), 1):
            it = futs[fut]
            results[it[0]] = fut.result()
            r = results[it[0]]
            status = f"wr_rc={r.get('wr_rc')}"
            print(f"[{k}/{len(items)}] {it[0]:03d} {r.get('category','?'):9s} {it[1][1][:70]:70s} {status}", flush=True)
    (ROOT / "results.json").write_text(json.dumps(results, indent=1, ensure_ascii=False))
    with (ROOT / "summary.csv").open("w") as f:
        cols = [
            "i", "category", "url", "http_code", "content_type", "original_bytes", "orig_words",
            "wr_rc", "wr_time", "md_bytes", "md_words", "md_vs_orig_ratio", "sentence_recall",
            "word_recall", "word_precision", "md_has_title", "md_headings", "md_fences", "md_tables",
            "md_images", "md_links", "md_lists", "leaks", "leak_total", "render_rc", "render_bytes",
            "render_words", "render_ansi_seqs", "render_leftover_heading", "render_leftover_fence",
            "render_raw_link_syntax", "render_word_recall", "stderr",
        ]
        import csv
        w = csv.writer(f)
        w.writerow(cols)
        for r in results:
            r = r or {}
            row = []
            for c in cols:
                v = r.get(c, "")
                if c == "wr_time":
                    v = r.get("wr_time", r.get("time", ""))
                if c == "stderr":
                    v = (r.get("stderr") or r.get("render_stderr") or "")[:200].replace("\n", " ")
                row.append(v)
            w.writerow(row)
    errs = [r for r in results if r.get("wr_rc") not in (0, None)]
    print(f"\nDONE {len(results)} urls in {time.time()-t0:.0f}s; errores wr: {len(errs)}")


if __name__ == "__main__":
    main()
