# wr

A reader and browser for your terminal. `wr` fetches a page, extracts the
article and opens it in an animated reader built with
[Bubble Tea](https://github.com/charmbracelet/bubbletea): **properly formatted
Markdown** (real headings, bold, lists, tables — no `#` or `**` in sight),
**per-language syntax highlighting**, and a full browsing experience — click
links, go back and forward, an address bar that also searches the web, history,
bookmarks. Everything is customizable from inside the program. The colors come
from your terminal's own 16-color ANSI palette — one accent plus its partner,
and the shades picked for your background — so it follows your theme (gruvbox,
nord, …) with zero configuration. Pages that only exist once their JavaScript
runs are rendered for you when a Chromium or Chrome is installed, and `wr` tells
you when a newer version is out and updates itself on request.

```
wr https://example.com/some-article
wr rust async book          # not an address? it is searched on the web
wr                          # the start page: your bookmarks and recent pages
```

```
  ⣿ Building a Realtime Forum in Go                                        ⡇
  ⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀  ⡇
                                                                           ⡇
  Build a 100% CLI realtime forum in Go: TCP, TLS, deduplicated           ⡇
  broadcast and resilient reconnects, zero dependencies.                   ⡇
                                                                           ⠿
⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀
 o open  / search  H back  L fwd  f links  t index  r reload  m menu  q quit    ⣷⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀   6%
```

*(a text capture: in your terminal the colors come from your theme)*

## What you get

### Reading

- **Formatted Markdown.** Headings with a mark and a rule, bold, italic, inline
  code, nested lists, quotes, tables, strikethrough and task lists, with the
  Markdown syntax gone.
- **Code with color.** Highlighted per language (chroma) inside a labeled frame.
  The language is read from `language-xx` classes, `data-language` and the
  Sphinx and MediaWiki conventions. When a site ships none (Shiki and other
  build-time highlighters), a small conservative detector colors the block only
  if it is sure and labels the frame `(guessed)`; otherwise it stays plain.
- **Clean extraction.** Menus, footers and cookie walls go, and so do the things a
  browser would not show you either: hidden elements, heading permalinks (`¶`),
  "skip to content" links, breadcrumbs above the title and empty links. Cards
  (a link wrapping a title and a description) keep the title as the link.
- **Pages drawn by JavaScript.** A single-page app that sends an empty shell is
  recognised and rendered through an installed headless Chromium/Chrome, in a
  few seconds; with no browser you get its own summary under a notice that says
  what to do.
- **Search as you type** (`/`), smart-case, highlighted matches, `n` / `N` to
  jump between them. **Section index** (`t`) and heading-to-heading jumps.
- **Select and copy.** Drag to select; it is copied when you let go. Double click
  selects a word, triple click a line. Code blocks copy **without** their frame
  and margin, and paragraphs wrapped on screen come out as the single line they
  were written as.

### Browsing

- **Links are clickable.** Click one, or press `Tab` / `Shift+Tab` to focus
  links (the footer shows where it goes) and `Enter` to open. Or press `f` and
  every link on screen gets a short label: type it to follow that link, no mouse
  needed.
- **Back and forward** (`H` / `L`, `Backspace`, `Alt+←/→`, or your mouse's back
  and forward buttons). Going back returns you to the exact spot you left, and
  it is instant because pages are cached.
- **An address bar** (`o`). Type an address, a local file, or plain words:
  words search the web (DuckDuckGo by default; Wikipedia, GitHub or your own
  engine are one setting away). It suggests your bookmarks and history as you
  type. `localhost:3000`, private addresses and `.local` names default to `http://`.
- **History** (`v`) and **bookmarks** (`'`, toggle the current page with `*`).
  Filter by typing, open with `Enter`, remove with `Ctrl+D`. A star in the footer
  shows a bookmarked page.
- **A start page** (`~`, or just run `wr`) listing your bookmarks and recent
  pages as links. Make any page your homepage from Settings.
- **In-page links** (`#section`) jump to the matching heading. Search-engine
  redirect links go straight to the real target.
- **More than HTML.** Markdown files and READMEs render as Markdown, plain text
  and JSON appear in a code block, and local files work (`wr notes.md`). Things
  it cannot show (images, PDFs) say so, and `x` opens the page in your real
  browser.
- **Failures are safe and explained.** A page that fails to load never replaces
  the one you are reading, and a slow load can be overtaken by a newer one. The
  error says what happened in plain words: who answered a 403 (`“Acceso web
  restringido”` is your proxy, `“Just a moment…”` is Cloudflare), no such host,
  connection refused; a reset or a timeout is retried once. Links are resolved
  against where a redirect really landed.

### Make it yours

Everything below is changed **from the menu**, live, and saved for you:

- **Settings** (`,` or menu → Settings): over 30 options in groups — layout
  (width, centering, spacing, heading rules, code frames, guessing code
  languages, links mode, braille, accent color), footer and scrollbar (each part
  separately, including turning the footer off), mouse, every animation
  separately plus scroll speed, browsing (search engine, homepage, rendering
  JavaScript pages, update checks and the what's-new window, history), cache. `←`/`→` change a value, `d` restores its
  default, and the selected option's help is shown underneath.
- **Keyboard shortcuts** (menu → Keyboard shortcuts): rebind any key. Select an
  action, press `Enter`, press the new key. `a` adds another key to an action,
  `x` unbinds, `d` restores the defaults. The footer hints follow your bindings.
- Prefer a file? It is `~/.config/wr/config.toml`, fully commented. Both ways
  edit the same file.

### Stays current

The version is always in sight (footer, menu title, start page). `wr` looks for
a newer release by itself — in the reader only, a few seconds after it starts, at
most once every 12 hours, one request to `api.github.com` — and tells you once,
then keeps `↑ v0.7.1 (U)` in the footer.

- **`U`** (or menu → *Update wr / what's new…*) opens a small window with the
  release notes; `Enter` updates, `Esc` leaves it for later. You can keep reading
  while it downloads, then restart `wr`.
- **`wr --update`** does the same from the shell, printing the notes first.
- **A "what's new" window of its own** is available but **off by default**
  (`whats_new`): when on, it opens by itself once per version when an update is
  found (never over a search or the address bar) and again on the first launch
  after you update.
- The download comes from the project's GitHub release, its SHA-256 is checked,
  the new binary must run and report the right version, and only then does it
  replace the installed one; a failure leaves your `wr` untouched.
- Off with `update_check = false`, the Settings entry or `WR_NO_UPDATE_CHECK=1`.

### Motion

Built on Charm's [harmonica](https://github.com/charmbracelet/harmonica) spring
physics, and designed to cost nothing at rest:

- **Spring scrolling.** Every jump (`G`, `]`, a search hit, a link, the mouse
  wheel) glides with a critically damped spring: no bounce. A one-line step lands
  in about 60 ms and a 300-line jump in about 330 ms. The Braille scrollbar and
  progress bar follow the fractional position, so they move smoothly too.
- **Page reveal**, **shutter panels**, a **braille loading wave** and **quick
  notices** that type themselves out and vanish when you press a key.
- **Zero idle cost.** Frames are only scheduled while something is moving.

Each animation has its own switch, and `animations = false` makes everything
instant.

The reader on video: `media/*.tape` are [VHS](https://github.com/charmbracelet/vhs)
tapes and `scripts/capture.sh` regenerates the recorded frames on the
[docs site](https://sazardev.github.io/wr/) — see
[media/README.md](media/README.md).

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

**The quick way** (Linux and macOS, no Go needed): download the right binary
from the [latest release](https://github.com/sazardev/wr/releases/latest),
verify it and put it in `~/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/sazardev/wr/main/install.sh | sh
```

(`INSTALL_DIR=/usr/local/bin` to choose where, `WR_VERSION=v0.1.0` to pick a
version. The script checks the SHA-256 checksum before installing. Read it
first if you like: [install.sh](install.sh).)

**Or by hand:** download `wr_<os>_<arch>.tar.gz` for your system from the
[releases page](https://github.com/sazardev/wr/releases/latest) (`linux` or
`darwin`, `amd64` or `arm64`), check it against `checksums.txt`, and unpack the
`wr` binary anywhere on your `PATH`.

**With Go 1.26 or newer:**

```sh
go install github.com/sazardev/wr@latest
```

**From source:**

```sh
git clone https://github.com/sazardev/wr.git
cd wr
go build -ldflags="-s -w" -o ~/.local/bin/wr .
```

Release binaries are static (no libc needed) and about 14 MB (chroma bundles all
its lexers). Tested on Linux (WSL2, CachyOS) with Alacritty and Go 1.27; the
macOS and arm64 builds are cross-compiled and have not been run by me. Windows:
use WSL.

**Updating:** `wr --update`, or `U` inside the reader (see [Stays current](#stays-current)).

**New to wr? Read the [guide](docs/GUIDE.md)** (also on the
[docs site](https://sazardev.github.io/wr/docs/guide/), one page per section,
with six palettes — [the design system](docs/DESIGN.md) describes them): how to
follow links, go back and forward, search the web, use history and bookmarks,
select and copy, and make it yours.

Braille needs a font with those glyphs (most modern monospace fonts have them,
e.g. Nerd Fonts). If yours does not, turn off *Braille decoration* in Settings.

## Usage

```sh
wr                 # the start page
wr URL             # open an address
wr some words      # anything that is not an address or a file is searched
wr notes.md        # a local file
wr -L URL          # links hidden for this session
wr --md URL        # print the Markdown as is, for pipes (always fetches fresh)
wr --fresh URL     # ignore the cache and download again
wr --clear-cache   # delete the cache
wr --config        # create (if missing) and print the config file path
wr --update        # check for a newer release and install it
wr --version       # print the version
wr --mcp           # serve the MCP server for AI agents (docs/MCP.md)
wr --eval          # run the eval scenarios, print a JSON report
```

When the output is not a terminal (`wr URL | less -R`) it prints the rendered
document and exits, with no interface. `NO_COLOR` (or `TERM=dumb`) makes that
print uncolored, and `COLUMNS` sets its width.

Environment: `WR_BROWSER` (the Chromium/Chrome used for pages drawn by
JavaScript), `WR_NO_UPDATE_CHECK`, `NO_COLOR`, `COLUMNS`, `VISUAL`/`EDITOR` (for
*Edit the config file…*) and the `XDG_*_HOME` directories. The guide has the
[full list](docs/GUIDE.md#environment-variables).

### Shortcuts

These are the defaults; every one can be rebound.

| Key | Action |
|---|---|
| `o` | open an address or search the web |
| `H` `L` / `Backspace` | back / forward |
| `Tab` `Shift+Tab` | focus the next / previous link; `Enter` opens it |
| `f` | label the links on screen, type a label to follow it |
| `v` / `'` / `*` | history / bookmarks / bookmark this page |
| `~` | the start page |
| `x` | open this page in your browser |
| `j` `k` / `↓` `↑` | one line |
| `space` `b` | page down / up |
| `d` `u` | half page |
| `g` `G` | top / bottom |
| `]` `[` | next / previous section |
| `t` | section index |
| `/` | search in the page (`enter` accepts, `esc` cancels and returns where you were) |
| `n` `N` | next / previous match |
| `r` | reload (or apply the newer version already downloaded) |
| `y` | copy the current selection again |
| `,` | settings |
| `U` | update wr / what's new |
| `m` | menu (`1`–`9` and `0` pick an entry directly) |
| `esc` | clear the search, the selection and the link focus |
| `q` | quit |

`Ctrl+C` always quits.

### Mouse

| Gesture | Action |
|---|---|
| click a link | open it |
| wheel | scroll |
| drag | select, and copy on release (drag past the top or bottom edge to scroll) |
| double click | select a word and copy it |
| triple click | select a line and copy it |
| back / forward buttons | back / forward |
| `Shift` + drag | your terminal's own selection, if you prefer it |

Copying uses two routes at once: OSC 52 (works over SSH and in most modern
terminals) and a system clipboard tool when one is available (`wl-copy`,
`xclip`, `xsel`, `pbcopy`, and on WSL PowerShell or `clip.exe`), so it works
even where the terminal ignores OSC 52. Turn off *Mouse support* in Settings to
leave the mouse entirely to your terminal, or *Copy on select* to copy with `y`.

## Configuration

Use **Settings** in the program; it saves for you. The file
(`wr --config` creates it) has everything commented and is equivalent. It is
optional, and a broken file never stops you from reading (a warning is shown
and the defaults are used).

| Key | Default | What it does |
|---|---|---|
| `width` | `100` | max width of the reading column (40–400); `0` = full width |
| `center` | `true` | center that column |
| `spacing` | `1` | blank lines between blocks: 0 compact, 1 normal, 2 airy |
| `heading_rules` | `true` | a rule under level 1 and 2 headings |
| `code_frame` | `true` | label and frame around code blocks |
| `guess_code_lang` | `true` | infer the language of unlabeled code when wr is sure (shown as `(guessed)`) |
| `render_js` | `true` | render script-drawn pages with an installed Chromium/Chrome |
| `update_check` | `true` | look for a newer wr in the background (at most every 12 hours) |
| `whats_new` | `false` | a small window with the release notes when there is an update |
| `links` | `"footnotes"` | `footnotes`, `inline` or `hidden` (links stay clickable in all of them) |
| `braille` | `true` | braille decoration; `false` = plain glyphs |
| `accent` | `"blue"` | color of panels, keys, marks and bars, and the one the headings walk: blue, cyan, green, magenta, yellow, red, gray, white (your terminal's own colors) or orange, violet, teal, pink, lime, slate (fixed presets) |
| `scrollbar` | `true` | scrollbar on the right |
| `footer` | `true` | the footer row |
| `footer_rule` | `true` | the thin rule above the footer |
| `footer_hints` | `true` | shortcut hints on the left |
| `footer_progress` | `"both"` | `both`, `percent`, `bar` or `off` |
| `footer_chips` | `true` | version, bookmark star, update available, search count |
| `mouse` | `true` | wheel, clicks and drag-to-copy |
| `copy_on_select` | `true` | copy when you release the mouse |
| `wheel_lines` | `3` | lines per wheel notch |
| `animations` | `true` | master switch for all animations |
| `anim_scroll` `anim_reveal` `anim_panels` `anim_loading` `anim_notices` | `true` | each animation on its own |
| `scroll_speed` | `"normal"` | `slow`, `normal` or `fast` |
| `search_engine` | `"duckduckgo"` | `duckduckgo`, `wikipedia`, `github`, or your own URL with `%s` |
| `homepage` | `""` | page to open when you run `wr` with no address; empty = the start page |
| `history` | `true` | remember the pages you visit |
| `history_limit` | `1000` | how many visits to keep |
| `cache` | `true` | keep pages on disk |
| `cache_days` | `30` | days a page stays cached |
| `background_refresh` | `true` | refresh a cached page while you read it |
| `[keys]` | — | only the key bindings you changed |

### Where things live

| Path | What |
|---|---|
| `~/.config/wr/config.toml` | settings and key bindings |
| `~/.cache/wr/` | the page cache |
| `~/.local/state/wr/history.jsonl` | your history (one JSON line per visit) |
| `~/.local/state/wr/bookmarks.json` | your bookmarks |
| `~/.local/state/wr/update.json` | the newest release seen and which version was announced |

All directories are `0700` and files `0600` (the config is `0644`). **Privacy:**
the cache and history store the addresses and content of what you read. Turn
history and the cache off in Settings, or clear them from the History panel and
the menu.

## Cache

A page you already read opens **instantly** from the cache, and is refreshed in
the background while you read.

- If a newer version arrives, the footer shows `● new version (r)` and does not
  change the text under your eyes; `r` applies it.
- The cache is only used in the interactive view. With `--md` or in a pipe the
  page is always downloaded fresh (and the cache is updated).
- If you quit before the refresh finishes, it is discarded without leaving
  anything half-written.
- Clear it from the menu (this page or everything), with `wr --clear-cache`, or
  let it expire after `cache_days`.

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

It uses Bubble Tea **v2**: it asks the terminal for its background color at
startup and moves the palette to the half of the 16 colors that reads on it —
the bright one (94) on a dark background, the plain, darker one (34) on a light
one. A terminal that does not answer (some multiplexers) keeps the default,
which suits a dark one.

## How it works

1. Downloads the page (system certificate store, TLS always verified, a reset or
   a timeout retried once), checks its type and converts it to UTF-8. Relative
   links are resolved against the address the page really came from after any
   redirect.
2. Takes `<article>` (or `<main>`, or `<body>`) and drops menus, footers, forms,
   scripts, what the page hides, heading permalinks, empty and skip links,
   navigation above the title, and rows that are just links.
3. If the result is nearly empty and the page looks drawn by JavaScript, it looks
   for a text version, renders the page in a headless Chromium/Chrome when one is
   installed, or falls back to the page's own summary (`<noscript>`,
   description, JSON-LD) with a notice.
4. Reads each block's language (or guesses it, labelled, when it is sure) and
   converts to Markdown with
   [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown).
5. Parses the Markdown with [goldmark](https://github.com/yuin/goldmark) and
   draws it with a custom renderer into self-contained ANSI lines, recording
   where every link sits.
6. [Bubble Tea](https://github.com/charmbracelet/bubbletea) handles keys, mouse,
   resizing and the footer; [harmonica](https://github.com/charmbracelet/harmonica)
   drives the springs. In the background, and only here, it checks for a newer
   release.

## Limits

- **Heuristic extraction.** It takes `<article>`/`<main>`; on unusual pages it
  may keep noise or cut content.
- **JavaScript pages need a browser.** `wr` has no JavaScript engine of its own:
  pages drawn by scripts are rendered through an installed Chromium/Chrome (a few
  seconds, a headless instance with its sandbox on). Without one you get the
  page's summary and a notice. Logins, forms, sites that turn away automated
  browsers and content that appears after a click are out of reach; `x` opens
  the page in your real browser. Search engines that need JS will not work as
  the engine setting; the presets do.
- **In-page `#links` match headings only**, by their text (the way most sites
  generate ids), not arbitrary element ids.
- **The language comes from the HTML, or from a careful guess.** A block the page
  does not label is colored only when the detector (Go, Python, JavaScript,
  TypeScript, JSON, Bash, SQL, Rust, Java, C#) is sure, and marked `(guessed)`;
  a short or ambiguous snippet stays plain. `mermaid` and other languages
  without a chroma lexer are never colored.
- **Networks that intercept.** A company proxy can answer for a site; the error
  shows the title of whoever replied.
- **Self-update is for release installs.** A build without a release version
  (`go build` between tags) never checks; a system-wide install needs `sudo`;
  there are no Windows release builds (use WSL).
- **Search works per line**: a phrase split across two lines of the rendered text
  is not found.
- **Selection copies what is drawn** (a heading keeps its mark, a table its
  borders), except for the code-block frame and the soft wraps described above.
- Very unusual Markdown (embedded HTML, footnotes) is simplified.

## Development

```sh
git clone https://github.com/sazardev/wr
cd wr
sh scripts/setup-hooks.sh   # once per clone: gofmt, vet and tests on commit
go test -race ./...
```

The test suite covers the renderer (including link positions), search,
selection and copy, cache, config and settings, key bindings, the persistent
store, URL handling, the whole browsing model — clicking, back and forward,
failures, overtaken loads, the address bar, history and bookmarks — and the newer
parts: pages drawn by JavaScript (a fake browser stands in for Chromium, including
the timeout that must kill its children), the language guesser, and the update
path end to end (a stub GitHub, a signed-by-checksum archive, a binary that is
refused when it is corrupt or reports the wrong version). It is deterministic: no
timing, no real network, no terminal emulator.

Extraction quality is audited against real pages: `scripts/qa/audit.py` runs a
list of 70+ developer-documentation URLs through a `wr` binary and flags
backslash line breaks, empty links, repeated lines, template leaks, thin results
and the opening lines of every page (round 3 in
[`docs/QA-FINDINGS.md`](docs/QA-FINDINGS.md) came from it).

Performance lives in the benchmarks (`docs/BENCHMARKS.md`): extraction, render,
cache vs no cache, and the per-key cost of the model, all local and
deterministic. `scripts/bench.sh` keeps the baseline and fails on regressions:

```sh
go test -run '^$' -bench . -benchmem
sh scripts/bench.sh baseline   # capture the before
sh scripts/bench.sh compare    # after: exits 1 on a regression over 25%
```

`wr --eval` runs 9 offline scenarios (107 checks) as a JSON report, exit 1 on
failure — the same contract `scripts/qa/mcp_smoke.py` checks over the MCP
server. The map of the whole codebase, the pipeline and every verification
surface is in [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Security

Page content is untrusted. Before display, control characters (including `ESC`)
are stripped, so a page cannot inject escape sequences into your terminal. This
also applies to what is read back from the cache, and an entry whose header does
not match the requested URL is discarded. Only `http`, `https` and `file` links
are followed; `mailto:`, `javascript:` and similar are refused with a notice.

The self-update downloads only from `github.com/sazardev/wr/releases/download/`
(any other address is refused), checks the archive against the release's
`checksums.txt` and runs the new binary before replacing the installed one. That
is the same trust as `install.sh`: it protects against corrupt or tampered
downloads, not against a compromised release.

## Contributing

Contributions are welcome — issues and pull requests. Commits follow
[Conventional Commits](https://www.conventionalcommits.org/); releases are cut
automatically by [release-please](https://github.com/googleapis/release-please)
from those commits, so the changelog and the tags always match what shipped.
Every PR is reviewed by [@sazardev](https://github.com/sazardev) and needs a
green CI run. See [CONTRIBUTING.md](CONTRIBUTING.md) for the details.

## License

[MIT](LICENSE). Use it, modify it, fork it, ship it, sell it — no restrictions.
The only thing asked in return is that the copyright notice stays, so the
project keeps its recognition.
