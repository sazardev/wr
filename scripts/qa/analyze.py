#!/usr/bin/env python3
# Analyzes results.json: aggregates, worst cases, boilerplate leaks, render.
import json
import os
import statistics as st
from collections import Counter, defaultdict
from pathlib import Path

ROOT = Path(os.environ.get("QA_ROOT", "qa-out")).resolve()
R = json.loads((ROOT / "results.json").read_text())

def ok(r):
    return r.get("wr_rc") == 0 and r.get("md_bytes", 0) > 0

cats = defaultdict(list)
for r in R:
    cats[r["category"]].append(r)

print("=== RESUMEN GLOBAL ===")
total = len(R)
curlfail = [r for r in R if not str(r.get("http_code", "")).startswith("2")]
wrfail = [r for r in R if r.get("wr_rc") != 0]
okr = [r for r in R if ok(r)]
print(f"urls={total}  curl_no2xx={len(curlfail)}  wr_err={len(wrfail)}  wr_ok={len(okr)}")
# fallos que curl sí obtuvo (bloqueo al cliente Go / tipo no soportado)
print("\n--- curl OK pero wr falla (bloqueo/limite/formato) ---")
for r in R:
    if str(r.get("http_code", "")).startswith("2") and r.get("wr_rc") != 0 and "cannot show" not in r.get("stderr", ""):
        print(f"{r['i']:03d} {r['category']:9s} curl={r['http_code']} wr={r.get('wr_rc')} {r.get('stderr','')[:70]:70s} {r['url'][:60]}")
print("\n--- formato no soportado (esperado) ---")
for r in R:
    if "cannot show" in r.get("stderr", ""):
        print(f"{r['i']:03d} {r['stderr'][:70]} {r['url'][:60]}")
print("\n--- curl tampoco pudo (URL rota / muro) ---")
for r in curlfail:
    print(f"{r['i']:03d} {r['category']:9s} http={r.get('http_code')} wr_rc={r.get('wr_rc')} {r['url'][:75]}")

print("\n=== POR CATEGORIA (solo extracciones válidas) ===")
print(f"{'categoria':10s} {'n':>3s} {'ok':>3s} {'ratio_med':>9s} {'recall_med':>10s} {'leak_med':>8s} {'sin_titulo':>10s} {'vacios':>6s}")
for c in sorted(cats):
    rs = cats[c]
    o = [r for r in rs if ok(r)]
    if not o:
        print(f"{c:10s} {len(rs):3d}   0  -        -          -        -        -")
        continue
    ratio = [r.get("md_vs_orig_ratio", 0) for r in o]
    rec = [r.get("sentence_recall") for r in o if r.get("sentence_recall") is not None]
    leaks = [r.get("leak_total", 0) for r in o]
    noT = [r for r in o if not r.get("md_has_title")]
    empty = [r for r in o if r.get("md_words", 0) < 50]
    print(f"{c:10s} {len(rs):3d} {len(o):3d} {st.median(ratio):9.2f} {st.median(rec) if rec else 0:10.2f} {st.median(leaks):8.0f} {len(noT):10d} {len(empty):6d}")

print("\n=== PEORES 25 POR RECALL DE CONTENIDO (sentences) [ok y >=300 palabras originales] ===")
cand = [r for r in okr if (r.get("orig_words", 0) >= 300 and r.get("sentence_recall") is not None)]
cand.sort(key=lambda r: r["sentence_recall"])
for r in cand[:25]:
    print(f"{r['i']:03d} {r['category']:9s} rec={r['sentence_recall']:.2f} ratio={r.get('md_vs_orig_ratio',0):.2f} md_words={r.get('md_words',0):6d} orig_words={r.get('orig_words',0):6d} leaks={r.get('leak_total',0):3d} {r['url'][:65]}")

print("\n=== MAS 'BASURA' (leaks) sobre ok, >=300 palabras ===")
cand = [r for r in okr if r.get("orig_words", 0) >= 300]
cand.sort(key=lambda r: -r.get("leak_total", 0))
for r in cand[:20]:
    print(f"{r['i']:03d} {r['category']:9s} leaks={r.get('leak_total',0):3d} {str(r.get('leaks',{}))[:110]} {r['url'][:55]}")

print("\n=== SALIDAS CASI VACIAS (md_words<40) con original >=200 palabras ===")
for r in okr:
    if r.get("md_words", 0) < 40 and r.get("orig_words", 0) >= 200:
        print(f"{r['i']:03d} {r['category']:9s} md_words={r.get('md_words')} orig={r.get('orig_words')} ct={r.get('content_type','')[:30]} {r['url'][:65]}")

print("\n=== SIN TITULO (# al inicio) sobre ok ===")
notitle = [r for r in okr if not r.get("md_has_title")]
print(f"{len(notitle)}/{len(okr)} = {len(notitle)/max(1,len(okr))*100:.0f}%")
for r in notitle[:20]:
    print(f"{r['i']:03d} {r['category']:9s} h1={r.get('o_h1')} {r['url'][:70]}")

print("\n=== RESTO DE SINTAXIS MARKDOWN EN EL RENDER (solo ok) ===")
bad = [r for r in okr if r.get("render_leftover_heading") or r.get("render_leftover_fence") or r.get("render_raw_link_syntax")]
print(f"con restos: {len(bad)}/{len(okr)}")
for r in bad[:20]:
    print(f"{r['i']:03d} h={r.get('render_leftover_heading')} f={r.get('render_leftover_fence')} l={r.get('render_raw_link_syntax')} {r['url'][:65]}")

print("\n=== COBERTURA RENDER vs ORIGINAL (word recall) bajas ===")
cand = [r for r in okr if r.get("render_word_recall") is not None and r.get("orig_words", 0) >= 300]
cand.sort(key=lambda r: r["render_word_recall"])
for r in cand[:15]:
    print(f"{r['i']:03d} {r['category']:9s} rend_rec={r['render_word_recall']:.2f} md_rec={r.get('word_recall',0):.2f} {r['url'][:65]}")

print("\n=== LENGUAJES DE FENCES DETECTADOS (top) ===")
langs = Counter()
for r in okr:
    for l in r.get("md_fence_langs", []):
        langs[l] += 1
print(langs.most_common(30))

print("\n=== FENCES SIN LENGUAJE ===")
nolang = sum(1 for r in okr if r.get("md_fences", 0) > len(r.get("md_fence_langs", [])))
withf = [r for r in okr if r.get("md_fences", 0) > 0]
print(f"paginas con fences: {len(withf)}; de ellas con algún fence sin lenguaje: {nolang}")

print("\n=== TIEMPOS ===")
t = [r.get("time", 0) for r in R if r.get("wr_rc") == 0]
print(f"wr --md mediana={st.median(t):.2f}s p90={sorted(t)[int(len(t)*.9)]:.2f}s max={max(t):.2f}s")
rt = [r.get("render_time", 0) for r in R if r.get("render_rc") == 0]
print(f"render mediana={st.median(rt):.3f}s p90={sorted(rt)[int(len(rt)*.9)]:.3f}s max={max(rt):.2f}s")
