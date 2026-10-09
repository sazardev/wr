#!/usr/bin/env python3
# ansi_to_svg: a captured screen (raw ANSI bytes) -> one flat SVG frame.
#
# The palette is the reader's own 16 ANSI colors mapped to the "wr" theme tokens
# (docs/style.css), so a frame and a page look like the same machine. No filter,
# no gradient: text with fill colors, over a flat background.
import argparse
import html
import re
from pathlib import Path

# 16 ANSI colors + the bright half, as the wr theme resolves them.
FG = {
    30: "#07090d", 31: "#ff6b6b", 32: "#2ee66f", 33: "#f2c94c",
    34: "#4da3ff", 35: "#cba6f7", 36: "#7dd3fc", 37: "#c9d3e3",
    90: "#5f7089", 91: "#ff8787", 92: "#63e6a0", 93: "#ffd866",
    94: "#74b6ff", 95: "#dcb9ff", 96: "#a5e3ff", 97: "#eef3fa",
}
BG = {"40": "#07090d", "100": "#0d1117"}
CELL_W, CELL_H = 8, 17
PAD = 14

SGR = re.compile(rb"\x1b\[([0-9;]*)m")


def color(code, bold=False):
    if code == 0:
        return None
    if code in (1, 22):
        return None  # bold: no weight stack, the glyphs are already uniform
    if 30 <= code <= 37:
        return FG[code + 60] if bold else FG[code]
    if 90 <= code <= 97:
        return FG[code]
    return None


def to_svg(raw: bytes, title="", font=""):
    fg = "#c9d3e3"
    bold = False
    lines = [{"y": 0, "spans": []}]
    x = 0
    row = 0
    i = 0
    spans = lines[0]["spans"]

    def push(text):
        if text:
            spans.append({"x": x * CELL_W, "text": text, "fg": fg})

    while i < len(raw):
        b = raw[i]
        if b == 0x1B:
            m = SGR.match(raw, i)
            if m:
                codes = m.group(1).decode().split(";") if m.group(1) else ["0"]
                for c in codes:
                    n = int(c or 0)
                    if n == 0:
                        fg, bold = "#c9d3e3", False
                    elif n == 1:
                        bold = True
                    else:
                        col = color(n, bold)
                        if col:
                            fg = col
                i = m.end()
                continue
            j = i + 1
            if j < len(raw) and raw[j:j + 1] == b"[":
                k = j + 1
                while k < len(raw) and not (0x40 <= raw[k] <= 0x7E):
                    k += 1
                i = k + 1
                continue
            i += 1
            continue
        if b == 0x0A:
            push("")
            row += 1
            x = 0
            lines.append({"y": row, "spans": []})
            spans = lines[-1]["spans"]
            i += 1
            continue
        if b == 0x0D:
            i += 1
            continue
        if b < 0x20:
            i += 1
            continue
        ch, ln = "?", 1
        try:
            ch = raw[i:].decode("utf-8")[0]
            ln = len(ch.encode("utf-8"))
        except Exception:
            pass
        push(ch)
        x += 1
        i += ln
    push("")

    rows = (row + 1) * CELL_H + PAD * 2
    cols = 100 * CELL_W + PAD * 2
    out = [
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{cols}" height="{rows}" '
        f'viewBox="0 0 {cols} {rows}" role="img" aria-label="{html.escape(title)}">',
        f'<rect width="{cols}" height="{rows}" fill="#07090d"/>',
    ]
    if title:
        out.append(f'<text x="{PAD}" y="{PAD - 4}" font-size="10" fill="#5f7089" '
                   f'font-family="ui-monospace,monospace" letter-spacing="1">{html.escape(title)}</text>')
    for l in lines:
        if not l["spans"]:
            continue
        y = PAD + l["y"] * CELL_H
        # merge adjacent runs with the same fill into one <text> (site weight)
        runs = []
        for sp in l["spans"]:
            if not sp["text"]:
                continue
            if runs and runs[-1]["fg"] == sp["fg"] and runs[-1]["x"] + len(runs[-1]["text"]) * CELL_W == sp["x"]:
                runs[-1]["text"] += sp["text"]
            else:
                runs.append({"x": sp["x"], "text": sp["text"], "fg": sp["fg"]})
        for r in runs:
            out.append(
                f'<text x="{PAD + r["x"]}" y="{y}" fill="{r["fg"]}" '
                f'font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" '
                f'font-size="13" xml:space="preserve">{html.escape(r["text"])}</text>')
    out.append("</svg>")
    return "\n".join(out) + "\n"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("frame")
    ap.add_argument("--title", default="")
    ap.add_argument("--out", required=True)
    a = ap.parse_args()
    raw = Path(a.frame).read_bytes()
    # a frame may contain many redraws: keep the last screen by cutting at the
    # last clear-screen, then splitting on it
    cut = raw.rfind(b"\x1b[2J")
    if cut >= 0:
        raw = raw[cut:]
    Path(a.out).write_text(to_svg(raw, a.title), encoding="utf-8")
    print(f"  wrote {a.out} ({Path(a.out).stat().st_size} bytes)")


if __name__ == "__main__":
    main()
