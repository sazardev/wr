#!/usr/bin/env python3
# Protocol smoke test for the MCP server (wr --mcp): spawns the binary, speaks
# newline-delimited JSON-RPC over its stdio and checks the whole surface an
# agent uses -- initialize, tools/list, the reading tools, and a full headless
# session (open, keys, type, screen, close). Needs no Go toolchain: point WR at
# any wr binary.
import json
import os
import subprocess
import sys
import tempfile
import time
from pathlib import Path

WR = os.environ.get("WR", "wr")
SANDBOX = Path(tempfile.mkdtemp(prefix="wr-mcp-smoke-"))
ENV = dict(os.environ,
           XDG_CONFIG_HOME=str(SANDBOX / "config"),
           XDG_CACHE_HOME=str(SANDBOX / "cache"),
           XDG_STATE_HOME=str(SANDBOX / "state"),
           WR_MCP_DEBUG=os.environ.get("WR_MCP_DEBUG", ""))

EXPECTED_TOOLS = [
    "wr_read", "wr_render", "wr_links", "wr_resolve_input",
    "wr_session_open", "wr_session_key", "wr_session_type", "wr_session_screen",
    "wr_session_close", "wr_session_list",
    "wr_history", "wr_bookmarks", "wr_bookmark_toggle", "wr_bookmark_delete",
    "wr_history_delete", "wr_history_clear",
    "wr_cache_stats", "wr_cache_clear", "wr_config_get", "wr_config_set",
]


class Client:
    def __init__(self):
        self.proc = subprocess.Popen([WR, "--mcp"], stdin=subprocess.PIPE,
                                     stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                                     text=True, bufsize=1, env=ENV)
        self.next_id = 0

    def call(self, method, params=None, timeout=20):
        self.next_id += 1
        rid = self.next_id
        req = {"jsonrpc": "2.0", "id": rid, "method": method}
        if params is not None:
            req["params"] = params
        self.proc.stdin.write(json.dumps(req) + "\n")
        self.proc.stdin.flush()
        deadline = time.time() + timeout
        while time.time() < deadline:
            line = self.proc.stdout.readline()
            if not line:
                raise RuntimeError("server closed the stream")
            resp = json.loads(line)
            if resp.get("id") != rid:
                continue  # a notification or a stale line
            if "error" in resp:
                raise RuntimeError(f"rpc error for {method}: {resp['error']}")
            return resp.get("result")
        raise TimeoutError(f"no answer for {method} in {timeout}s")

    def notify(self, method):
        self.proc.stdin.write(json.dumps({"jsonrpc": "2.0", "method": method}) + "\n")
        self.proc.stdin.flush()

    def tool(self, name, args=None):
        return self.call("tools/call", {"name": name, "arguments": args or {}})

    def close(self):
        self.proc.stdin.close()
        self.proc.wait(timeout=10)


class Report:
    def __init__(self):
        self.failed = 0

    def check(self, name, ok, detail=""):
        mark = "ok  " if ok else "FAIL"
        print(f"{mark} {name}" + ("" if ok else f": {detail}"))
        if not ok:
            self.failed += 1

    def result(self, res):
        sc = res.get("structuredContent", {})
        if res.get("isError"):
            raise RuntimeError(f"tool failed: {res.get('content')}")
        return sc


def main():
    page = SANDBOX / "page.md"
    page.write_text("# Smoke Page\n\nBody text to read.\n\nA [link](https://x.test/t).\n")
    rep = Report()
    c = Client()
    try:
        init = c.call("initialize", {"protocolVersion": "2025-06-18",
                                     "clientInfo": {"name": "mcp_smoke", "version": "0"}})
        c.notify("notifications/initialized")
        rep.check("handshake version", init.get("protocolVersion") == "2025-06-18", init)
        rep.check("serverInfo", init.get("serverInfo", {}).get("name") == "wr", init)
        rep.check("capabilities", "tools" in init.get("capabilities", {}), init)
        rep.check("instructions", bool(init.get("instructions")))

        listed = c.call("tools/list")["tools"]
        names = [t["name"] for t in listed]
        rep.check("tool count", len(names) == len(EXPECTED_TOOLS), names)
        rep.check("tool set", sorted(names) == sorted(EXPECTED_TOOLS), names)
        rep.check("tools documented", all(t.get("description") and t.get("inputSchema") for t in listed))

        sc = rep.result(c.tool("wr_read", {"target": str(page)}))
        rep.check("read title", sc.get("title") == "Smoke Page", sc)
        rep.check("read body", "Body text to read." in sc.get("markdown", ""), sc)

        sc = rep.result(c.tool("wr_links", {"target": str(page)}))
        links = sc.get("links", [])
        rep.check("one link", sc.get("count") == 1 and links and links[0]["url"] == "https://x.test/t", sc)

        sc = rep.result(c.tool("wr_resolve_input", {"input": "hello world"}))
        rep.check("resolves search", sc.get("search") is True and "duckduckgo" in sc.get("url", ""), sc)

        sc = rep.result(c.tool("wr_session_open", {"target": str(page), "width": 60, "height": 12}))
        sid = sc.get("session_id", "")
        rep.check("session opens", bool(sid) and "Smoke Page" in sc.get("screen", ""), sc)
        rep.check("session state", sc.get("state", {}).get("mode") == "read", sc)

        sc = rep.result(c.tool("wr_session_key", {"session_id": sid, "key": "/"}))
        rep.check("key opens search", sc.get("state", {}).get("mode") == "search", sc)
        sc = rep.result(c.tool("wr_session_type", {"session_id": sid, "text": "Body"}))
        rep.check("type fills input", "Body" in sc.get("screen", ""), sc)
        rep.result(c.tool("wr_session_key", {"session_id": sid, "key": "esc"}))
        rep.check("key esc returns", rep.result(
            c.tool("wr_session_screen", {"session_id": sid}))["state"]["mode"] == "read")

        rep.check("session listed", any(s["session_id"] == sid for s in
                  rep.result(c.tool("wr_session_list"))["sessions"]))
        try:
            raw = c.tool("wr_session_key", {"session_id": sid, "key": "no-such-key"})
            rep.check("bad key errors", bool(raw.get("isError")), raw)
        except RuntimeError:
            rep.check("bad key errors", True)
        rep.result(c.tool("wr_session_close", {"session_id": sid}))
        again = c.tool("wr_session_close", {"session_id": sid})
        rep.check("closed session gone", bool(again.get("isError")), again)

        cfg = rep.result(c.tool("wr_config_get"))
        rep.check("config readable", "width" in cfg.get("config", {}), cfg)
        rep.result(c.tool("wr_config_set", {"settings": {"width": 90, "animations": False}}))
        cfg = rep.result(c.tool("wr_config_get"))
        rep.check("config saved", cfg["config"]["width"] == 90 and cfg["config"]["animations"] is False, cfg)
    finally:
        c.close()

    print(f"\n{'FAILED' if rep.failed else 'PASSED'}: {rep.failed} failing checks (sandbox {SANDBOX})")
    return 1 if rep.failed else 0


if __name__ == "__main__":
    sys.exit(main())
