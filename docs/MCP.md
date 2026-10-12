# The agent harness: `wr --mcp`

`wr --mcp` turns the binary into an [MCP](https://modelcontextprotocol.io)
server: JSON-RPC 2.0 over stdin/stdout, one message per line, no HTTP and no
sockets. An AI agent gets three things:

- **the reader**, headless: the same extraction, renderer and layout the binary
  shows, as data;
- **the reader, interactive**: the real Bubble Tea model driven by keys, with
  the screen and the state returned after every key, without a terminal;
- **the store**: history, bookmarks, cache and config, the same files the app
  reads.

Everything runs in one process with the same code paths as the CLI, so what an
agent reads is what you see. Nothing is exported from `package main`, and the
server is a mode of the binary for that reason: `load`, `renderDoc`, `newModel`
and the store stay private, and there is a single binary to ship.

## Connect it

**opencode** (`opencode.json` at the project root, or `~/.config/opencode/`):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "wr": {
      "type": "local",
      "command": ["/usr/local/bin/wr", "--mcp"],
      "enabled": true
    }
  }
}
```

**Claude Code**:

```sh
claude mcp add wr -- /usr/local/bin/wr --mcp
```

**Claude Desktop** (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "wr": { "command": "/usr/local/bin/wr", "args": ["--mcp"] }
  }
}
```

**Any client**: it is `stdio`. Set `WR_MCP_DEBUG=1` to log the traffic to
stderr (stdout carries the protocol and nothing else).

The server works with no configuration at all; it uses your `config.toml`
settings, your search engine and your store paths. To give an agent a sandbox,
point the XDG variables elsewhere before launching it — everything (config,
cache, history, bookmarks) follows:

```sh
XDG_CONFIG_HOME=/tmp/agent/config XDG_CACHE_HOME=/tmp/agent/cache \
XDG_STATE_HOME=/tmp/agent/state wr --mcp
```

## The tools

| Tool | What it does |
|---|---|
| `wr_read` | fetch a page, file or search words → the extracted Markdown (always fresh), with the title. `strip_links` keeps the text and drops the URLs. |
| `wr_render` | the same page rendered like the reader shows it: plain text (or `ansi`) wrapped to a width. |
| `wr_links` | the links of a page, one per destination, in reading order: text, href, absolute url and kind (`web`, `file`, `anchor`, `unsupported`). |
| `wr_resolve_input` | what the address bar would make of an entry (url, file or search) without fetching. |
| `wr_session_open` | open a page in a headless interactive session (`width`, `height`, `fresh`). |
| `wr_session_key` | press a key: `j k g G t n N / o f m , ? q y r v ' * ~ x H L [ ] d u b`, `enter`, `esc`, `tab`, `shift+tab`, `space`, arrows, `pgup/pgdown`, `home/end`, `backspace`, `delete`, `ctrl+d/f/u/b`, `alt+left/right`. `repeat` presses it n times. |
| `wr_session_type` | type text (the address bar after `o`, the search line after `/`, a link label after `f`). |
| `wr_session_screen` | the current screen plus the state (see below). `ansi` keeps the colors. |
| `wr_session_close` / `wr_session_list` | close a session, or list the open ones. |
| `wr_history` | the history, newest first, with `limit` and `filter`. |
| `wr_bookmarks` | the bookmarks, newest first. |
| `wr_bookmark_toggle` / `wr_bookmark_delete` | add/remove a bookmark (the target resolves like an address-bar entry). |
| `wr_history_delete` / `wr_history_clear` | remove entries, or the whole history. |
| `wr_cache_stats` / `wr_cache_clear` | pages and bytes on disk; delete the cache. |
| `wr_config_get` | the effective config (file over defaults) and the path of `config.toml`. |
| `wr_config_set` | change settings by name (`width`, `accent`, `search_engine`, `cache_days`, …), clamped and saved like the Settings panel does. |

Every tool returns the same envelope: a `content` text block (compact JSON)
plus `structuredContent`, so a client can parse it without a text round-trip.
A failure comes back with `isError: true` and the reason in the text.

### Session state

```json
{
  "session_id": "s1",
  "state": {
    "url": "file:///tmp/a.html", "title": "Page A", "mode": "read",
    "loading": false, "error": "", "toast": "", "from_cache": false,
    "bookmarked": false, "query": "", "matches": 0,
    "back": ["file:///tmp/start.html"], "forward": [],
    "links": 3, "focused_link": "file:///tmp/b.html",
    "scroll": {"y": 12, "total": 84, "visible": 29, "percent": 21}
  },
  "screen": "…the 30 rows the terminal would show…"
}
```

`mode` is the reader's own: `read`, `search`, `hints`, `omni` (address bar),
`menu`, `confirm`, `toc`, `settings`, `keys`, `list`. After a key that loads a
page the state may still say `loading: true` — the load runs on a per-session
goroutine pool exactly like the app, so the next `wr_session_screen` shows it.

### A session, start to finish

```
wr_session_open  {target: "https://example.com"}         → screen of the page
wr_session_key   {session_id, key: "/"}                  → mode: search
wr_session_type  {session_id, text: "Example"}          → the query on screen
wr_session_key   {session_id, key: "enter"}             → matches: 2
wr_session_key   {session_id, key: "n"}                 → the next match
wr_session_key   {session_id, key: "esc"}               → mode: read
wr_session_key   {session_id, key: "f"}                 → hints: a label per link
wr_session_type  {session_id, text: "a"}                → follows that link
wr_session_screen {session_id}                          → the new page + state
wr_session_close {session_id}
```

## QA with agents

The harness is also how a QA agent works the page corpus in `scripts/qa/`:

1. **`wr --eval`** first: 9 deterministic scenarios (input resolution,
   extraction, rendering, links, headless navigation, search, the store, the
   cache, config) as a JSON report, exit 1 on any failure. It runs fully
   offline and touches nothing outside a temp `HOME`:

   ```sh
   go run . --eval | jq '{passed, failed, bad: [.scenarios[] | select(.ok == false) | .name]}'
   ```

2. **`scripts/qa/mcp_smoke.py`**: a client-side smoke of the protocol itself
   (handshake, tools/list, read, links, a whole session) against any `wr`
   binary, with no Go toolchain:

   ```sh
   WR=$PWD/wr python3 scripts/qa/mcp_smoke.py
   ```

3. then the interesting part, per URL of the corpus: `wr_read` to judge the
   extraction, `wr_render` to judge the layout, `wr_links` to catch broken or
   wrong destinations, `wr_session_*` to walk a flow (open → follow → back)
   and compare the screen against what should be there. Findings go into
   `docs/QA-FINDINGS.md` like the rest of the project.

## Limits

- Sessions live in process memory: closing the server closes them; ids restart
  on the next run.
- `wr_read`/`wr_render`/`wr_links` always fetch fresh (the pipe-mode contract:
  no cache reads); they do write the cache for next time when it is enabled.
- One client at a time per process: the stdio server is single-threaded per
  request, sessions run their loads concurrently.
- The screen is the model rendered headless: identical to the terminal at the
  session's `width`/`height`, but without a real terminal's replies (the
  background color detection keeps its default, the dark half of the palette).
