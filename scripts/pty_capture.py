#!/usr/bin/env python3
# Capture a scene of the real program in a PTY: open a page, send keys, dump the
# final screen. Deterministic by construction: fixed size, fixed config (see
# media/demo.toml), no animation.
#
# Usage: pty_capture.py --wr ./wr --config media/demo.toml --page media/demo.html \
#                       --scene scenes.json:ui --out /tmp/cap [--frame N]
import argparse
import fcntl
import json
import os
import pty
import re
import select
import signal
import struct
import termios
import subprocess
import sys
import time
from pathlib import Path

ANSI = re.compile(r"\x1b\[[0-9;?]*[A-Za-z]|\x1b_[^\x1b]*\x1b\\|\x1b\][^\x07]*\x07")


# ---------------------------------------------------------------- VT screen --
class Screen:
    """Minimal VT emulator: enough of cursor addressing, clears and SGR to
    reconstruct what a terminal would show."""

    def __init__(self, w=100, h=30):
        self.w, self.h = w, h
        self.grid = [[" "] * w for _ in range(h)]
        self.cx = self.cy = 0

    def feed(self, buf: bytes):
        i = 0
        while i < len(buf):
            b = buf[i]
            if b == 0x1B:
                j = i + 1
                if j < len(buf) and buf[j:j + 1] == b"[":
                    k = j + 1
                    while k < len(buf) and not (0x40 <= buf[k] <= 0x7E):
                        k += 1
                    if k >= len(buf):
                        break
                    seq, fin = buf[j + 1:k].decode("latin1"), chr(buf[k])
                    if fin == "H":
                        parts = seq.split(";")
                        self.cy = max(0, min(self.h - 1, (int(parts[0] or 1)) - 1))
                        self.cx = max(0, min(self.w - 1, (int(parts[1] or 1) if len(parts) > 1 else 1) - 1))
                    elif fin == "J":
                        self.grid = [[" "] * self.w for _ in range(self.h)]
                    elif fin == "K":
                        for x in range((0 if seq in ("", "0", "1") else self.cx), self.w):
                            self.grid[self.cy][x] = " "
                    elif fin in "ABCD":
                        n = int(seq) if seq.isdigit() else 1
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
                if j < len(buf) and buf[j:j + 1] in (b"]", b"_"):
                    e = [c for c in (buf.find(b"\x07", j), buf.find(b"\x1b\\", j)) if c >= 0]
                    if not e:
                        break
                    i = min(e) + (1 if min(e) == buf.find(b"\x07", j) else 2)
                    continue
                i += 1
                continue
            if b == 0x0D:
                self.cx = 0
                i += 1
                continue
            if b == 0x0A:
                self.cy = min(self.h - 1, self.cy + 1)
                i += 1
                continue
            if b < 0x20:
                i += 1
                continue
            ch, ln = "?", 1
            try:
                ch = buf[i:].decode("utf-8")[0]
                ln = len(ch.encode("utf-8"))
            except Exception:
                pass
            if 0 <= self.cy < self.h and 0 <= self.cx < self.w:
                self.grid[self.cy][self.cx] = ch
            self.cx += 1
            i += ln
        return self

    def text(self):
        return "\n".join("".join(l).rstrip() for l in self.grid)


def keys_to_bytes(steps):
    out = []
    for step in steps:
        out.append(step.encode().decode("unicode_escape").encode("latin1"))
    return out


def capture(wr, config, home, page, scene, width, height, frame_ms=90):
    env = dict(os.environ)
    env.update(HOME=str(home), XDG_CONFIG_HOME=str(home / ".config"),
               XDG_CACHE_HOME=str(home / ".cache"),
               XDG_STATE_HOME=str(home / ".local" / "state"),
               TERM="xterm-256color", COLUMNS=str(width), LINES=str(height))
    for v in ("WAYLAND_DISPLAY", "DISPLAY"):
        env.pop(v, None)

    pid, fd = pty.fork()
    if pid == 0:
        fcntl.ioctl(0, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
        os.execvpe(str(wr), [str(wr), str(page)], env)

    try:
        fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
    except OSError:
        pass  # the child already sized its own tty

    acc = []

    def drain(quiet=0.6, total=8.0):
        end, last = time.time() + total, time.time()
        while time.time() < end and time.time() - last < quiet:
            r, _, _ = select.select([fd], [], [], 0.1)
            if r:
                try:
                    c = os.read(fd, 1 << 16)
                except OSError:
                    break
                if not c:
                    break
                acc.append(c)
                last = time.time()

    drain(quiet=(scene.get("wait") or 1.0), total=10.0)
    for k in keys_to_bytes(scene.get("keys", [])):
        os.write(fd, k)
        time.sleep(0.35)
        drain(quiet=0.4, total=3.0)
    time.sleep(scene.get("wait", 0.5) * 0.5)
    drain(quiet=0.8, total=6.0)
    raw = b"".join(acc)
    try:
        os.write(fd, b"q")
    except OSError:
        pass
    time.sleep(0.15)
    try:
        os.kill(pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    try:
        os.close(fd)
    except OSError:
        pass
    sc = Screen(width, height).feed(raw)
    return sc, raw


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--wr", required=True)
    ap.add_argument("--config", required=True)
    ap.add_argument("--page", required=True)
    ap.add_argument("--scene", required=True, help="scenes.json path or path:name")
    ap.add_argument("--out", required=True)
    ap.add_argument("--width", type=int, default=100)
    ap.add_argument("--height", type=int, default=30)
    a = ap.parse_args()

    spec, name = a.scene, None
    if ":" in a.scene:
        spec, name = a.scene.split(":", 1)
    scenes = json.loads(Path(spec).read_text())["scenes"]
    if name:
        scenes = [s for s in scenes if s["name"] == name]

    home = Path(a.out) / "home"
    cfgdir = home / ".config" / "wr"
    cfgdir.mkdir(parents=True, exist_ok=True)
    (cfgdir / "config.toml").write_text(Path(a.config).read_text())

    outdir = Path(a.out) / "frames"
    outdir.mkdir(parents=True, exist_ok=True)
    for s in scenes:
        sc, raw = capture(Path(a.wr).resolve(), a.config, home, a.page if a.page != "LOCAL" else str(Path(a.page).resolve()), s, a.width, a.height)
        (outdir / f"{s['name']}.ansi").write_bytes(raw)
        (outdir / f"{s['name']}.txt").write_text(sc.text() + "\n")
        print(f"  captured {s['name']} ({len(raw)} bytes, {len(sc.text().splitlines())} lines)")


if __name__ == "__main__":
    main()
