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
(`javascript:`, `file:` from remote pages), or leaking history/cache contents.
The guarantees the reader relies on are described in the README's
[Security](README.md#security) section.

## Supported versions

Only the latest release is supported; fixes ship in the next tagged release.
