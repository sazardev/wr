# wr

A reader mode for your terminal. `wr` fetches a page, extracts the article and
opens it in an animated reader built with [Bubble Tea](https://github.com/charmbracelet/bubbletea):
**properly formatted Markdown** (real headings, bold, lists, tables — no `#` or
`**` in sight), **per-language syntax highlighting**, reading progress, search,
a section index, an on-disk cache, and a spring-physics UI drawn partly in
**Braille**. Everything uses your terminal's own 16-color ANSI palette, so it
follows your theme (gruvbox, nord, …) with zero configuration.

```
wr https://example.com/some-article
```

```
  ⣿ Building a Realtime Forum in Go                                        ⡇
  ⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀  ⡇
                                                                           ⡇
  Build a 100% CLI realtime forum in Go: TCP, TLS, deduplicated           ⡇
  broadcast and resilient reconnects, zero dependencies.                   ⡇
                                                                           ⠿
⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀
 ⠿  example.com  ↺ cached 2h ago                    ⣷⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀   6%
 / search  n N next  [ ] sections  t index  r reload  m menu  q quit
```

*(a text capture: in your terminal the colors come from your theme)*

## What you get

- **Formatted Markdown.** Headings with a mark and a rule, bold, italic, inline
  code, nested lists, quotes, tables, strikethrough and task lists, with the
  Markdown syntax gone.
- **Code with color.** Highlighted per language (chroma) inside a labeled frame.
  The language is read from `data-language` or `language-xx` classes.
- **A footer that stays out of the way.** Host, cache state, reading
  **percentage** and a Braille progress bar. It adapts to the terminal: wide
  terminals get the full thing, narrow ones drop what does not fit (host,
  extra shortcuts, bar size), and very short ones collapse it to a single row.
- **Search** as you type (`/`), smart-case, highlighted matches, `n` / `N` to
  jump between them.
- **Section index** (`t`) and heading-to-heading jumps (`[` / `]`).
- **Menu** (`m`): reload, clear the cache (this page or everything, with
  confirmation), switch the links mode, style and animations, open the settings,
  show the shortcuts.
- **Disk cache.** A page you already read opens instantly and is refreshed in
  the background; if a newer version arrives the footer tells you and `r`
  applies it.
- **Motion with physics.** See below.

## Animations

Built on Charm's [harmonica](https://github.com/charmbracelet/harmonica) spring
physics, and designed to cost nothing at rest:

- **Spring scrolling.** Every jump (`G`, `]`, a search hit, the mouse wheel)
  glides with a critically damped spring: no bounce. A one-line step lands in
  about 60 ms and a 300-line jump in about 330 ms. The Braille scrollbar and
  progress bar follow the fractional position, so they move smoothly too.
- **Page reveal.** A page wipes in from the top behind a bright scan line.
- **Shutter panels.** The menu, index and dialogs open from their middle row.
- **Loading wave.** The first visit shows a traveling Braille ripple with a
  color gradient while the page downloads.
- **Typewriter notices.** Toasts type themselves out.
- **Zero idle cost.** Frames are only scheduled while something is moving. When
  the screen settles, no tick is requested and the program uses no CPU.

Turn them off with `animations = false` (or from the menu); everything then
snaps instantly.

## Why it exists

I wanted to read documentation and technical articles without leaving the
terminal and without losing the highlighted code. What I tried first did not do
it:

| Tool | What happened |
|---|---|
| `w3m` / `lynx` | They render the HTML, but code stays plain: they ignore `style` and `<font color>`. |
| `trafilatura --markdown` | Good extraction, but it drops the language of code blocks (bare ```), leaving nothing to highlight with. |
| `bat -l markdown` | Colors the Markdown, but **not** the code inside fences. |

`wr` glues the pieces together and keeps the key piece of data: each block's
language.

## Install

With Go 1.26 or newer:

```sh
go install github.com/sazardev/wr@latest
```

or from source:

```sh
git clone https://github.com/sazardev/wr.git
cd wr
go build -ldflags="-s -w" -o ~/.local/bin/wr .
```

The binary is about 14 MB when built with `-ldflags="-s -w"` (19 MB with a plain
`go install`; chroma bundles all its lexers) and needs nothing else at runtime. Tested on Linux (WSL2, CachyOS) with Alacritty and Go 1.27; not
tested on macOS or native Windows.

Braille needs a font with those glyphs (most modern monospace fonts have them,
e.g. Nerd Fonts). If yours does not, set `braille = false`.

## Usage

```sh
wr URL            # open the reader
wr -L URL         # no links (text only)
wr --md URL       # print the Markdown as is, for pipes (always fetches fresh)
wr --fresh URL    # ignore the cache and download again
wr --clear-cache  # delete the cache
wr --config       # create (if missing) and print the config file path
wr page.html      # works with a local file too
```

When the output is not a terminal (`wr URL | less -R`) it prints the rendered
document and exits, with no interface.

### Shortcuts

| Key | Action |
|---|---|
| `j` `k` / `↓` `↑` | one line |
| `space` `b` | page down / up |
| `d` `u` | half page |
| `g` `G` | top / bottom |
| `]` `[` | next / previous section |
| `t` | section index |
| `/` | search (`enter` accepts, `esc` cancels and returns where you were) |
| `n` `N` | next / previous match |
| `r` | reload (or apply the newer version already downloaded) |
| `m` | menu (`1`–`9` and `0` pick an entry directly) |
| `esc` | clear the search |
| `q` | quit |

The mouse wheel scrolls too. While mouse support is on, hold `Shift` to select
text (or set `mouse = false`).

## Configuration

`wr --config` creates `~/.config/wr/config.toml` with everything commented.
Everything is optional; a broken file never stops you from reading (a warning is
shown and the defaults are used).

| Key | Default | What it does |
|---|---|---|
| `width` | `100` | max width of the reading column (40–200) |
| `center` | `true` | center that column |
| `braille` | `true` | Braille decoration; `false` = plain glyphs |
| `scrollbar` | `true` | scrollbar on the right |
| `footer` | `true` | footer with progress and shortcuts |
| `links` | `"footnotes"` | `footnotes` (numbered at the end), `inline` (URL next to the text) or `hidden` |
| `mouse` | `true` | mouse wheel |
| `animations` | `true` | spring scrolling, reveal, panel and loading animations |
| `cache_days` | `30` | days a page stays in the cache |

From the menu (`m`) you can open the file in your `$EDITOR`; the settings reload
when you close it. Changes to links, style and animations made in the menu last
for the session only.

## Cache

A page you already read opens **instantly** from `~/.cache/wr`, and is refreshed
in the background while you read.

- If a newer version arrives, the footer shows `● new version (r)` and does not
  change the text under your eyes; `r` applies it.
- The cache is only used in the interactive view. With `--md` or in a pipe the
  page is always downloaded fresh (and the cache is updated).
- If you quit before the refresh finishes, it is discarded without leaving
  anything half-written.
- Clear it from the menu (`m` → this page or everything), with
  `wr --clear-cache`, or let it expire after `cache_days`. Directory `0700`,
  files `0600`.
- **Privacy:** the cache stores the URLs and content of what you read.

## Speed

Measured with an article of ~36 KB of Markdown (1300 lines, 22 code blocks) on
WSL2:

| | Time |
|---|---|
| First paint (the loading wave appears) | ~30 ms |
| Cached visit: first text on screen | ~50–60 ms (the page then wipes in over ~140 ms) |
| First visit | ~0.4–0.7 s, almost all network |
| Render the whole document | ~18 ms |
| Re-render (resize, style or link change) | ~3 ms |

Code is highlighted in parallel, one goroutine per block, and the result is
memoized, so resizing the window never tokenizes again.

It uses Bubble Tea **v2**: v1 queries the terminal for its background color at
startup and, if the terminal does not answer (some multiplexers), waits up to
5 seconds.

## How it works

1. Downloads the page (system certificate store, TLS always verified) and
   converts it to UTF-8.
2. Takes `<article>` (or `<main>`, or `<body>`) and drops menus, footers, forms,
   scripts and rows that are just links.
3. Reads each block's language and converts to Markdown with
   [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown).
4. Parses the Markdown with [goldmark](https://github.com/yuin/goldmark) and
   draws it with a custom renderer into self-contained ANSI lines.
5. [Bubble Tea](https://github.com/charmbracelet/bubbletea) handles keys, mouse,
   resizing and the footer; [harmonica](https://github.com/charmbracelet/harmonica)
   drives the springs.

## Limits

- **Heuristic extraction.** It takes `<article>`/`<main>`; on unusual pages it
  may keep noise or cut content.
- **No JavaScript.** If the page draws its content with JS, it will come out
  empty or incomplete.
- **Links are not navigable**: they are listed at the end (or next to the text).
  For browsing, a terminal browser such as `w3m` is still the right tool.
- **The language depends on the HTML.** If the page does not declare it, the
  block is shown without color (it is not guessed). `mermaid` and other
  languages without a chroma lexer are not colored either.
- **Search works per line**: a phrase split across two lines of the rendered text
  is not found.
- Very unusual Markdown (embedded HTML, footnotes) is simplified.

## Development

```sh
go test -race ./...
```

The test suite covers the renderer, search, cache, config, the model (including
the spring and reveal animations driven with synthetic ticks) and the real
Bubble Tea renderer through Charm's terminal emulator.

## Security

Page content is untrusted. Before display, control characters (including `ESC`)
are stripped, so a page cannot inject escape sequences into your terminal. This
also applies to what is read back from the cache, and an entry whose header does
not match the requested URL is discarded.

## License

[MIT](LICENSE)
