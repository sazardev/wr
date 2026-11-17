# Security policy

## Reporting a vulnerability

Please do not open a public issue for security problems. Use GitHub's private
reporting instead:

**https://github.com/sazardev/wr/private-vulnerability-reporting** → *Report a vulnerability*

You will get a response as soon as possible, and a fix released with credit to
you unless you prefer otherwise.

## What counts

`wr` renders untrusted web pages in a terminal, so anything that lets a page
escape the reader is a security bug: terminal escape-sequence injection,
path traversal in the cache or config, following links that should be refused
(`javascript:`, `file:` from remote pages, a remote page's advertised alternate
pointing at a local file or a private address), or leaking history/cache
contents. The guarantees the reader relies on are described in the README's
[Security](README.md#security) section.

The MCP server (`wr --mcp`) runs with your permissions and exposes the store as
tools. When the client is not fully trusted, run it with `WR_MCP_READONLY=1`
(no writes, no history, no cache data) or point `XDG_CONFIG_HOME`,
`XDG_CACHE_HOME` and `XDG_STATE_HOME` at separate directories; see
[`docs/MCP.md`](docs/MCP.md#connect-it).

## Supported versions

Only the latest release is supported; fixes ship in the next tagged release.
