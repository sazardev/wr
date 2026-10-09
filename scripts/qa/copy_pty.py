#!/usr/bin/env python3
# Real copy test: runs wr inside a PTY, sends SGR mouse drags and double/triple
# clicks, then captures and decodes the OSC 52 clipboard payload. DISPLAY and
# WAYLAND_DISPLAY are cleared so only the OSC 52 path can produce output.
import base64
import json
import os
import pty
import re
import select
import signal
import time
from pathlib import Path

ROOT = Path(os.environ.get("QA_ROOT", "qa-out")).resolve()
WR = os.environ.get("WR", "wr")
HOME = ROOT / "home"
ENV = dict(os.environ, HOME=str(HOME), XDG_CONFIG_HOME=str(HOME / ".config"),
           XDG_CACHE_HOME=str(HOME / ".cache"), XDG_STATE_HOME=str(HOME / ".local" / "state"),
           TERM="xterm-256color", COLUMNS="80", LINES="30")
ENV.pop("WAYLAND_DISPLAY", None)
ENV.pop("DISPLAY", None)

LONG = "x = \"" + "a" * 130 + "\"  # tail"
DOC = f"""# Code copy test

Intro paragraph.

```python
def greet(name):
    print(f"hello {{name}}")
    {LONG}
    return True
```

End.
"""

ANSI = re.compile(rb"\x1b\[[0-9;?]*[A-Za-z]|\x1b_[^\x1b]*\x1b\\|\x1b\][^\x07]*\x07")


def plain(buf):
    return ANSI.sub(b"", buf).decode("utf-8", "replace")


def start():
    pid, fd = pty.fork()
    if pid == 0:
        os.execvpe(WR, [WR, str(ROOT / "copy-test.md")], ENV)
    return pid, fd


def drain(fd, until=b"return True", timeout=10.0):
    buf, end = b"", time.time() + timeout
    while until not in buf and time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.2)
        if r:
            try:
                c = os.read(fd, 65536)
            except OSError:
                break
            if not c:
                break
            buf += c
    return buf


def drain_quiet(fd, quiet=0.8, total=4.0):
    buf, end, last = b"", time.time() + total, time.time()
    while time.time() < end and time.time() - last < quiet:
        r, _, _ = select.select([fd], [], [], 0.15)
        if r:
            try:
                c = os.read(fd, 65536)
            except OSError:
                break
            if c:
                buf += c
                last = time.time()
    return buf


def m(b, x, y, rel=False):
    return f"\x1b[<{b};{x+1};{y+1}{'m' if rel else 'M'}".encode()


def clips(buf):
    out = []
    for mm in re.finditer(rb"\x1b\]52;[^;]*;([A-Za-z0-9+/=]*)\x07", buf):
        try:
            out.append(base64.b64decode(mm.group(1)).decode("utf-8", "replace"))
        except Exception:  # noqa: BLE001
            pass
    return out


def y_of(out, needle, nth=0):
    hits = 0
    for y, l in enumerate(plain(out).split("\n")):
        if needle in l:
            if hits == nth:
                return y
            hits += 1
    return None


def scenario(name, fn):
    pid, fd = start()
    try:
        out = drain(fd)
        r = {"scenario": name, "doc": "return True" in plain(out)}
        fn(fd, out, r)
        extra = drain_quiet(fd)
        r["toast"] = "copied" in plain(extra)
        r["osc52"] = clips(out + extra)
        return r
    finally:
        try:
            os.write(fd, b"q")
        except OSError:
            pass
        time.sleep(0.2)
        try:
            os.kill(pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        try:
            os.close(fd)
        except OSError:
            pass


def do_drag(fd, out, r):
    sc = Screen().feed(out)
    y0 = sc.find("def greet")[1]
    y1 = sc.find("return True")[1]
    r["ys"] = (y0, y1)
    fd and os.write(fd, m(0, 10, y0))
    time.sleep(0.15)
    os.write(fd, m(32, 30, (y0 + y1) // 2))
    time.sleep(0.15)
    os.write(fd, m(32, 30, y1))
    os.write(fd, m(0, 30, y1, rel=True))


def click(fd, x, y):
    os.write(fd, m(0, x, y))
    time.sleep(0.08)
    os.write(fd, m(0, x, y, rel=True))
    time.sleep(0.15)


def do_triple(fd, out, r):
    sc = Screen().feed(out)
    pos = sc.find("return True")
    r["y"] = pos[1]
    for _ in range(3):
        click(fd, max(pos[0], 1), pos[1])


def do_double(fd, out, r):
    sc = Screen().feed(out)
    pos = sc.find("name")
    x, y = max(pos[0], 1), pos[1]
    r["y"] = (y, x, sc.line(y).strip()[:40])
    click(fd, x, y)
    click(fd, x, y)


def do_long_tail(fd, out, r):
    sc = Screen().feed(out)
    y = sc.find("x = \"aaa")[1]
    y2 = y + 1
    r["ys"] = (y, y2)
    os.write(fd, m(0, 12, y))
    time.sleep(0.15)
    os.write(fd, m(32, 20, y2))
    time.sleep(0.15)
    os.write(fd, m(0, 20, y2, rel=True))




class Screen:
    """Emulador VT mínimo: grid 80x30 para localizar coordenadas reales."""
    def __init__(self, w=80, h=30):
        self.w, self.h = w, h
        self.grid = [[" "] * w for _ in range(h)]
        self.cx = self.cy = 0

    def feed(self, buf: bytes):
        i = 0
        while i < len(buf):
            b = buf[i]
            if b == 0x1b:  # escape
                j = i + 1
                if j < len(buf) and buf[j:j+1] == b"[":
                    k = j + 1
                    while k < len(buf) and not (0x40 <= buf[k] <= 0x7e):
                        k += 1
                    if k >= len(buf):
                        break
                    seq = buf[j + 1:k].decode("latin1")
                    fin = chr(buf[k])
                    if fin == "H":
                        parts = seq.split(";")
                        self.cy = max(0, min(self.h - 1, (int(parts[0]) if parts[0] else 1) - 1))
                        self.cx = max(0, min(self.w - 1, (int(parts[1]) if len(parts) > 1 and parts[1] else 1) - 1))
                    elif fin == "K":
                        for x in range(self.cx if seq.startswith("0") or seq == "" else 0, self.w):
                            self.grid[self.cy][x] = " "
                    elif fin == "J":
                        self.grid = [[" "] * self.w for _ in range(self.h)]
                    elif fin in "ABCD":
                        n = int(seq) if seq.isdigit() and seq else 1
                        if fin == "A":
                            self.cy = max(0, self.cy - n)
                        elif fin == "B":
                            self.cy = min(self.h - 1, self.cy + n)
                        elif fin == "C":
                            self.cx = min(self.w - 1, self.cx + n)
                        elif fin == "D":
                            self.cx = max(0, self.cx - n)
                    i = k + 1
                    continue
                if j < len(buf) and buf[j:j+1] in (b"]", b"_"):
                    end = buf.find(b"\x07", j)
                    end2 = buf.find(b"\x1b\\", j)
                    cands = [e for e in (end, end2) if e >= 0]
                    if not cands:
                        break
                    i = min(cands) + (1 if min(cands) == end else 2)
                    continue
                i += 1
                continue
            if b == 0x0d:
                self.cx = 0
                i += 1
                continue
            if b == 0x0a:
                self.cy = min(self.h - 1, self.cy + 1)
                i += 1
                continue
            if b < 0x20:
                i += 1
                continue
            try:
                ch = buf[i:].decode("utf-8")[0]
                ln = len(ch.encode("utf-8"))
            except Exception:
                ch, ln = "?", 1
            if 0 <= self.cy < self.h and 0 <= self.cx < self.w:
                self.grid[self.cy][self.cx] = ch
            self.cx += 1
            i += ln
        return self

    def line(self, y):
        return "".join(self.grid[y])

    def find(self, needle, nth=0):
        hits = 0
        for y in range(self.h):
            x = self.line(y).find(needle)
            if x >= 0:
                if hits == nth:
                    return x, y
                hits += 1
        return None


def main():
    (ROOT / "copy-test.md").write_text(DOC)
    res = [scenario("drag-block", do_drag),
           scenario("triple-click-line", do_triple),
           scenario("double-click-word", do_double),
           scenario("drag-wrapped-long-line", do_long_tail)]
    (ROOT / "copy-results.json").write_text(json.dumps(res, indent=1, ensure_ascii=False))
    for r in res:
        print("=====", r["scenario"], "doc:", r["doc"], "toast:", r.get("toast"), r.get("ys") or r.get("y"))
        for c in r["osc52"]:
            print(repr(c))


if __name__ == "__main__":
    main()
