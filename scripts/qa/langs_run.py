#!/usr/bin/env python3
# Language matrix: per lexer, HTML -> wr --md -> render. Measures fence
# detection, code fidelity (indentation included), color (non-dim SGR codes),
# frame label and wrapping. Needs lang-corpus.json (see langs_corpus.py).
import json
import os
import re
import subprocess
import sys
from html import escape
from pathlib import Path

ROOT = Path(os.environ.get("QA_ROOT", "qa-out")).resolve()
OUT = ROOT / "langs"
OUT.mkdir(exist_ok=True)
WR = os.environ.get("WR", "wr")
HOME = ROOT / "home"
ENV = dict(os.environ, HOME=str(HOME), XDG_CONFIG_HOME=str(HOME / ".config"),
           XDG_CACHE_HOME=str(HOME / ".cache"), XDG_STATE_HOME=str(HOME / ".local" / "state"))
ENV.pop("COLUMNS", None)
WIDTH = 100
ANSI_RE = re.compile(r"\x1b\[[0-9;]*m")
ANY_ANSI = re.compile(r"\x1b\[[0-9;]*[A-Za-z]|\x1b_[^\x1b]*\x1b\\")
GUTTER_RE = re.compile(r"^\x1b\[90m[│┆]\x1b\[0m ")


def run(cmd, timeout=25, env=None):
    try:
        p = subprocess.run(cmd, capture_output=True, timeout=timeout, env=env or ENV)
        return p.returncode, p.stdout, p.stderr
    except subprocess.TimeoutExpired:
        return 124, b"", b"timeout"


def fence_of(md: str):
    m = re.search(r"(?m)^(`{3,})([^\n`]*)\n(.*?)\n\1\s*$", md, re.S)
    if not m:
        m2 = re.search(r"(?m)^(`{3,})([^\n`]*)$", md)
        return (m2.group(2).strip() if m2 else None), None
    return m.group(2).strip(), m.group(3)


def analyze_render(rendered: bytes, source: str):
    txt = rendered.decode("utf-8", "replace")
    lines = txt.split("\n")
    body, cont = [], 0
    label, top, bottom = None, 0, 0
    chromatic, codes = set(), set()
    for ln in lines:
        plain = ANSI_RE.sub("", ANY_ANSI.sub("", ln))
        if plain.startswith("╭─"):
            top = 1
            label = plain[2:].strip()
        elif plain.startswith("╰─"):
            bottom = 1
        elif re.match(r"^\x1b\[90m[│┆]", ln):
            is_cont = re.match(r"^\x1b\[90m┆", ln) is not None
            core = ANSI_RE.sub("", ANY_ANSI.sub("", ln))
            if core.startswith("│ ") or core.startswith("┆ "):
                core = core[2:]
            for c in ANSI_RE.findall(ln):
                code = c[2:-1]
                if code and code != "0":
                    codes.add(code)
                    for part in code.split(";"):
                        if part in ("90", "0", "", "1", "3", "4", "9"):
                            continue
                        chromatic.add(part)
            if is_cont:
                cont += 1
                if body:
                    body[-1] += core
                else:
                    body.append(core)
            else:
                body.append(core)
    return {
        "frame": bool(top and bottom),
        "label": label,
        "reconstructed": "\n".join(body),
        "wrapped_lines": cont,
        "chromatic_codes": sorted(chromatic),
        "all_codes": sorted(codes),
    }


def m_top(ln):
    return False


def m_bottom(ln):
    return False


def normalize(s: str) -> str:
    return s.replace("\t", "    ")


def cmp_code(src, got):
    a, b = normalize(src).rstrip("\n"), got.rstrip("\n")
    if a == b:
        return True, ""
    # permitir diferencias solo de espacios finales
    al = [l.rstrip() for l in a.split("\n")]
    bl = [l.rstrip() for l in b.split("\n")]
    if al == bl:
        return True, "solo espacios finales"
    if al != bl:
        for i, (x, y) in enumerate(zip(al, bl)):
            if x != y:
                return False, f"linea {i+1}: esperado {x[:60]!r} obtenido {y[:60]!r}"
        return False, f"nº lineas {len(al)} vs {len(bl)}"
    return True, ""


def one(entry, idx):
    name, alias, src, code = entry["lexer"], entry["alias"], entry["source"], entry["code"]
    d = OUT / f"{idx:03d}"
    d.mkdir(exist_ok=True)
    html = ('<!doctype html><html><head><title>code</title></head><body><article><h1>Code</h1>\n'
            f'<pre class="language-{escape(alias)}"><code>{escape(code)}</code></pre>\n'
            "</article></body></html>")
    (d / "in.html").write_text(html)
    rc, md_b, err = run([WR, "--md", str(d / "in.html")])
    res = {"i": idx, "lang": name, "alias": alias, "source": src,
           "hl_rc": rc, "hl_err": err.decode()[-120:], "code_lines": len(code.split("\n")),
           "code_bytes": len(code)}
    md = md_b.decode("utf-8", "replace")
    (d / "wr.md").write_text(md)
    lang, body = fence_of(md)
    res["md_fence_lang"] = lang
    res["md_body_ok"] = body == code.rstrip("\n") if body is not None else False
    res["md_diff"] = "" if res["md_body_ok"] else (f"md!=" if body is not None else "sin fence")
    rc, rend, err = run([WR, str(d / "wr.md")], env=dict(ENV, COLUMNS=str(WIDTH)))
    (d / "render.bin").write_bytes(rend)
    res["render_rc"] = rc
    if rc == 0:
        r = analyze_render(rend, code)
        res.update({"frame": r["frame"], "label": r["label"], "wrapped": r["wrapped_lines"],
                    "chromatic": r["chromatic_codes"], "n_chromatic": len(r["chromatic_codes"]),
                    "n_codes": len(r["all_codes"]),
                    "render_body_ok": cmp_code(code, r["reconstructed"])[0],
                    "render_diff": cmp_code(code, r["reconstructed"])[1]})
        (d / "render.txt").write_text(ANSI_RE.sub("", rend.decode("utf-8", "replace")))
    return res


EDGES = [
    ("edge:tabs", "go", "package main\n\nfunc main() {\n\tif true {\n\t\tprintln(\"tab\")\n\t}\n}\n"),
    ("edge:fence-inside", "markdown", "Example:\n\n```go\nfmt.Println(\"hi\")\n```\n\nEnd.\n"),
    ("edge:ansi-inside", "bash", "echo -e \"\\x1b[31mred\\x1b[0m\"\nesc=$(printf '\\033[1m')\n"),
    ("edge:long-line", "python", "x = \"" + "a" * 260 + "\"\nprint(x)\n"),
    ("edge:blank-lines", "python", "a = 1\n\n\n\nb = 2\n"),
    ("edge:gutter-glyph", "text", "│ fake guttter line\n┆ continuation glyph\nnormal\n"),
    ("edge:unicode-code", "python", "# ünïcödé ñ 中文 العربية\ndef f():\n    return \"héllo 世界 🚀\"\n"),
    ("edge:no-lang", "", "plain code line\nsecond line\n"),
    ("edge:html-entities", "html", "<div class=\"a\">&amp; &lt;tag&gt; &#39;q&#39;</div>\n"),
    ("edge:crlf-md", "text", "line one\r\nline two\r\n"),
    ("edge:only-blank", "text", "\n\n\n"),
    ("edge:trailing-ws", "python", "x = 1   \ny = 2\t\n"),
]


def main():
    corpus = json.load(open(ROOT / "lang-corpus.json"))
    entries = [(i, e) for i, e in enumerate(corpus)]
    base = len(corpus)
    for k, (nm, lang, code) in enumerate(EDGES):
        entries.append((base + k, {"lexer": nm, "alias": lang, "source": "edge", "code": code}))
    results = [None] * len(entries)
    import concurrent.futures as cf
    with cf.ThreadPoolExecutor(max_workers=6) as ex:
        futs = {ex.submit(one, e, i): (pos, e) for pos, (i, e) in enumerate(entries)}
        for k, fut in enumerate(cf.as_completed(futs), 1):
            pos, e = futs[fut]
            try:
                results[pos] = fut.result()
            except Exception as ex2:  # noqa: BLE001
                results[pos] = {"i": e and pos, "lang": e["lexer"], "alias": e["alias"], "source": e["source"], "harness_error": repr(ex2)}
            if k % 40 == 0:
                print(f"  {k}/{len(entries)}", flush=True)
    (ROOT / "langs-results.json").write_text(json.dumps(results, indent=1, ensure_ascii=False))
    ok = [r for r in results if r.get("hl_rc") == 0]
    nocolor = [r for r in ok if r.get("n_chromatic", 0) == 0]
    nobody = [r for r in ok if not r.get("render_body_ok")]
    print(f"\n{len(results)} lenguajes; md ok={len(ok)}; sin color cromático={len(nocolor)}; cuerpo distinto={len(nobody)}")


if __name__ == "__main__":
    main()
