#!/usr/bin/env python3
# Refined metrics: how much of the root content (article/main/body) reaches the
# Markdown, language detection on the source <pre> blocks vs fences, and h1s
# present in the root but missing from the Markdown.
import json
import os
import re
from html import unescape
from pathlib import Path

ROOTD = Path(os.environ.get("QA_ROOT", "qa-out")).resolve()
P = ROOTD / "pages"
R = json.loads((ROOTD / "results.json").read_text())

JUNK = re.compile(r"(?i)(?:^|[\s_-])(?:toc|table-of-contents|breadcrumbs?|sidebar|cookie\w*)(?:$|[\s_-])")
LANG_RES = [
    re.compile(r"(?i)(?:^|\s)(?:language|lang|highlight-source|brush)[-:_ ]+([\w+#.-]+)"),
    re.compile(r"(?i)(?:^|\s)sourceCode\s+([\w+#-]+)"),
]
WORD = re.compile(r"[^\W_]+", re.UNICODE)


def words(s):
    return [w.casefold() for w in WORD.findall(s) if len(w) > 1]


def text_of(html):
    h = re.sub(r"(?is)<(script|style|noscript|svg|template)[^>]*>.*?</\1>", " ", html)
    h = re.sub(r"(?s)<!--.*?-->", " ", h)
    h = re.sub(r"<[^>]+>", " ", h)
    return re.sub(r"\s+", " ", unescape(h)).strip()


def first_span(html, tag):
    m = re.search(rf"(?is)<{tag}\b[^>]*>", html)
    if not m:
        return None
    depth, start = 0, m.start()
    for t in re.finditer(rf"(?is)</?{tag}\b[^>]*>", html[start:]):
        if t.group(0).startswith("</"):
            depth -= 1
            if depth == 0:
                return html[start:start + t.end()]
        else:
            depth += 1
    return html[start:]


def sentences(text, n=40):
    parts = re.split(r"(?<=[.!?])\s+", text)
    out = [p for p in parts if len(p) >= 50]
    if len(out) <= n:
        return out
    step = len(out) / n
    return [out[int(i * step)] for i in range(n)]


rows = []
for r in R:
    d = P / f"{r['i']:03d}"
    html = (d / "orig.html").read_text("utf-8", "replace") if (d / "orig.html").exists() else ""
    md = (d / "wr.md").read_text("utf-8", "replace") if (d / "wr.md").exists() else ""
    art, main = first_span(html, "article"), first_span(html, "main")
    if art:
        root, kind = art, "article"
    elif main:
        root, kind = main, "main"
    else:
        root, kind = first_span(html, "body") or html, "body"
    # simulate drops for root text: dropTags + header + junkclass
    sim = re.sub(r"(?is)<(script|style|noscript|nav|aside|footer|form|svg|button|iframe|template|dialog|select|input)\b[^>]*>.*?</\1>", " ", root)
    if kind != "article":
        sim = re.sub(r"(?is)<header\b[^>]*>.*?</header>", " ", sim)
    root_text = text_of(sim)
    md_norm = re.sub(r"\s+", " ", md).casefold()
    ss = sentences(root_text)
    found = sum(1 for s in ss if len(re.sub(r"\s+", " ", s).casefold()[:60]) >= 30 and re.sub(r"\s+", " ", s).casefold()[:60] in md_norm)
    root_recall = round(found / len(ss), 3) if ss else None

    ow, mw = set(words(root_text)), set(words(md))
    rw, prec = (round(len(ow & mw) / max(1, len(ow)), 3), round(len(ow & mw) / max(1, len(mw)), 3)) if ow else (None, None)

    # language detection on pre blocks of root
    pres = re.findall(r"(?is)<pre\b[^>]*>", root)
    h1s = [text_of(x) for x in re.findall(r"(?is)<h1\b[^>]*>(.*?)</h1>", root)]
    # h1 inside header?
    h1_in_header = len(re.findall(r"(?is)<header\b[^>]*>(?:(?!</header>).)*?<h1\b", root)) > 0

    rows.append({
        **{k: r.get(k) for k in ("i", "url", "category", "wr_rc", "md_words", "orig_words", "leaks", "leak_total",
                                  "md_fences", "md_fence_langs", "md_tables", "o_table", "o_img", "md_images", "sentence_recall")},
        "root_kind": kind, "root_words": len(words(root_text)), "root_recall": root_recall,
        "root_word_recall": rw, "md_precision": prec,
        "pre_blocks": len(pres), "h1_texts": h1s, "h1_in_header": h1_in_header,
        "md_starts_title": bool(re.match(r"\s*#\s+\S", md)),
        "title_in_md": bool(h1s) and any(t and t.casefold()[:30] in md_norm for t in h1s),
        "md_has_hash_line": bool(re.search(r"(?m)^#\s", md)),
    })

os.makedirs(ROOTD, exist_ok=True)
(ROOTD / "metrics2.json").write_text(json.dumps(rows, indent=1, ensure_ascii=False))

ok = [x for x in rows if x["wr_rc"] == 0 and x.get("md_words")]
print(f"páginas ok: {len(ok)}")
recs = [x["root_recall"] for x in ok if x["root_recall"] is not None]
import statistics as st
print(f"root_recall: mediana={st.median(recs):.2f}  p10={sorted(recs)[len(recs)//10]:.2f}  p90={sorted(recs)[len(recs)*9//10]:.2f}")
print(f"root_word_recall: mediana={st.median([x['root_word_recall'] for x in ok if x['root_word_recall'] is not None]):.2f}")
print("\n-- root_recall por categoria --")
from collections import defaultdict
cat = defaultdict(list)
for x in ok:
    if x["root_recall"] is not None:
        cat[x["category"]].append(x["root_recall"])
for c in sorted(cat):
    v = cat[c]
    print(f"  {c:14s} n={len(v):3d} mediana={st.median(v):.2f} p10={sorted(v)[max(0,len(v)//10)]:.2f} <0.3={(sum(1 for q in v if q<0.3)):3d}")

print("\n-- páginas con root_recall < 0.3 (pérdida seria) --")
for x in sorted([x for x in ok if (x["root_recall"] or 0) < 0.3], key=lambda x: x["root_recall"] or 0):
    rr = x['root_recall'] or 0
    print(f"  {x['i']:03d} {x['category']:14s} rec={rr:.2f} root={x['root_kind']:7s} root_w={x['root_words']:6d} md_w={x['md_words']:6d} h1_in_header={x['h1_in_header']} {x['url'][:55]}")

print("\n-- títulos: h1 de la raíz presente en md --")
bad = [x for x in ok if x["h1_texts"] and not x["title_in_md"]]
print(f"con h1 en raíz: {len([x for x in ok if x['h1_texts']])}; de ellos h1 ausente del md: {len(bad)}")
for x in bad[:15]:
    print(f"  {x['i']:03d} h1={str(x['h1_texts'])[:40]:42s} in_header={x['h1_in_header']} starts_title={x['md_starts_title']} hash_line={x['md_has_hash_line']} {x['url'][:50]}")

print("\n-- detección de lenguaje --")
tot_pre = sum(x["pre_blocks"] for x in ok)
tot_fence = sum(x["md_fences"] for x in ok)
tot_lang = sum(len(x["md_fence_langs"]) for x in ok)
print(f"<pre> en raíces: {tot_pre}; fences en md: {tot_fence}; con lenguaje: {tot_lang} ({tot_lang/max(1,tot_fence)*100:.0f}%)")
notitle = [x for x in ok if x["md_has_hash_line"] and not x["md_starts_title"] and x.get("root_recall") is not None]
print(f"md con línea '#' que NO es título al inicio (fallback de título bloqueado): {len(notitle)}")
