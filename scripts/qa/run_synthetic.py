#!/usr/bin/env python3
# Runs wr over the synthetic cases and saves --md plus the rendered view.
import os
import subprocess
from pathlib import Path

ROOT = Path(os.environ.get("QA_ROOT", "qa-out")).resolve()
SYN = ROOT / "synthetic"
OUT = ROOT / "synthetic-out"
OUT.mkdir(exist_ok=True)
WR = os.environ.get("WR", "wr")
HOME = ROOT / "home"
env = dict(os.environ, HOME=str(HOME), XDG_CONFIG_HOME=str(HOME / ".config"),
           XDG_CACHE_HOME=str(HOME / ".cache"), XDG_STATE_HOME=str(HOME / ".local" / "state"))
env.pop("COLUMNS", None)

for f in sorted(SYN.iterdir()):
    d = OUT / f.name
    d.mkdir(exist_ok=True)
    p = subprocess.run(["timeout", "-k", "2", "30", WR, "--md", str(f)], capture_output=True, timeout=40, env=env)
    (d / "wr.md").write_bytes(p.stdout)
    (d / "md.err").write_bytes(p.stderr)
    (d / "rc.txt").write_text(str(p.returncode))
    if p.returncode == 0 and p.stdout:
        r = subprocess.run(["timeout", "-k", "2", "30", WR, str(d / "wr.md")], capture_output=True, timeout=40,
                           env=dict(env, COLUMNS="80"))
        (d / "render.txt").write_bytes(r.stdout)
        (d / "render.err").write_bytes(r.stderr)
        (d / "render.rc").write_text(str(r.returncode))
    print(f"{f.name:28s} rc={p.returncode} md={len(p.stdout)}B err={p.stderr.decode()[:80].strip()}")
