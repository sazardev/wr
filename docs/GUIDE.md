# wr guide

How to read and browse the web from your terminal with `wr`. If you only want
the short version, run `wr`, press `o`, type an address, and press `m` for the
menu: everything else can be discovered from there.

- [Start](#start)
- [The screen](#the-screen)
- [Reading a page](#reading-a-page)
- [Following links](#following-links)
- [Going back and forward](#going-back-and-forward)
- [The address bar and searching the web](#the-address-bar-and-searching-the-web)
- [History, bookmarks and the start page](#history-bookmarks-and-the-start-page)
- [Searching inside a page](#searching-inside-a-page)
- [Selecting and copying](#selecting-and-copying)
- [Make it yours](#make-it-yours)
- [Rebinding keys](#rebinding-keys)
- [Local files, Markdown and other content](#local-files-markdown-and-other-content)
- [Using it with other tools](#using-it-with-other-tools)
- [Recipes](#recipes)
- [Troubleshooting](#troubleshooting)
- [Where wr keeps things](#where-wr-keeps-things)

## Start

Install (see the [README](../README.md#install) for every option):

```sh
curl -fsSL https://raw.githubusercontent.com/sazardev/wr/main/install.sh | sh
```

Then:

```sh
wr                        # the start page
wr https://go.dev/doc/    # open an address
wr rust async book        # anything that is not an address or a file is a web search
wr notes.md               # a local file
```

Quit with `q`. `Ctrl+C` also always quits.

## The screen

```
  ⣿ Page title                                                      ⡇   ← the page, with a scrollbar
  ⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀  ⡇
  Text of the page ...                                              ⠿
⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀   ← the rule
 o open  / search  H back  L fwd  f links  t index  m menu  q quit   ⣷⣀⣀⣀  6%   ← the footer
```

- **The page** is the article extracted from the website: menus, footers and
  ads are dropped, and what is left is drawn with formatting.
- **The scrollbar** on the right shows where you are (it moves smoothly while
  you scroll).
- **The footer** has your most useful shortcuts on the left and your reading
  progress on the right. It adapts to the window: a narrow window shows fewer
  shortcuts, and a very short one drops the rule above it.
- **Small notes** appear next to the progress when they matter: `★` (this page
  is bookmarked), `● new version (r)` (the site changed while you read), and
  `«word» 3/12` (a search is active).
- **Notices** such as "copied 57 characters" replace the shortcuts for about a
  second and disappear the moment you press a key or click.
- Everything uses your terminal's 16 colors, so it matches your theme. The
  accent color (panels, keys, bars, and the colors the headings walk) can be
  changed in Settings, and it has a partner color for the secondary highlights
  (the light end of the footer rule, the search count, the focused link).

## Reading a page

| Key | What it does |
|---|---|
| `j` `k` or `↓` `↑` | one line down / up |
| `space` `b` (or `PgDn` `PgUp`) | a page down / up |
| `d` `u` | half a page |
| `g` `G` | top / bottom |
| `]` `[` | next / previous section (heading) |
| `t` | the **section index**: pick a heading and jump to it |
| mouse wheel | scroll (3 lines per notch by default) |

Scrolling is smooth: it glides to where you are going instead of jumping. If
you prefer it instant, turn off *Smooth scrolling* in Settings.

Code blocks are highlighted by language and shown in a labeled frame. The
language comes from the page, so a block the site did not label is shown in
plain color.

## Following links

Three ways, use whichever you like:

1. **Click it.** Links are underlined and colored.
2. **Tab through them.** `Tab` focuses the next link (`Shift+Tab` the previous),
   and the footer shows where it goes. Press `Enter` to open it, `Esc` to let
   go.
3. **Type a label.** Press `f`: every link on the screen gets a short label
   (`a`, `s`, `d`, `f`, ...). Type the label to open that link. `Esc` cancels.
   This is the fastest way if your hands are on the keyboard.

Notes:

- How links are *displayed* is a setting (*Links*): **footnotes** (a small
  number after the link text and a list at the end of the page), **inline**
  (the address next to the text), or **hidden** (just the text). They are
  clickable in all three modes, and in footnotes mode the address list at the
  end is clickable too.
- A link to a section of the same page (`#section`) just scrolls there.
- Search engines wrap results in redirect links; `wr` goes straight to the real
  target.
- `mailto:`, `javascript:` and similar links are not followed: you get a notice
  saying so. Only `http`, `https` and local `file` links are opened.
- If a page fails to load, you stay on the page you were reading and get a
  notice. Nothing is lost.

## Going back and forward

`wr` remembers the pages of your session like a browser does.

| Key | What it does |
|---|---|
| `H` or `Backspace` or `Alt+←` | back |
| `L` or `Alt+→` | forward |
| your mouse's back / forward buttons | back / forward |

Going back puts you at **exactly the spot you left**, and it is instant because
the page comes from the cache. Opening something new from a page you went back
to discards the "forward" pages, as in any browser.

## The address bar and searching the web

Press `o`. Type and press `Enter`.

- An **address** (`example.com`, `https://example.com/page`,
  `localhost:8080/app`) is opened. You can leave out `https://`.
- A **file** (`./notes.md`, `~/docs/readme.html`, `/etc/hosts`) is opened.
- **Anything else is a web search.** `rust async book` searches for it.

The first row always shows what `wr` will do with what you typed ("Open ..." or
"Search the web for ..."). Below it are your **bookmarks (★)** and **history**
that match what you typed (every word you type must appear in the title or
address). Pick one with `↑` `↓` (or `Tab`) and press `Enter`.

| Key (in the address bar) | What it does |
|---|---|
| `↑` `↓`, `Tab` / `Shift+Tab` | choose a row |
| `Enter` | open it |
| `Ctrl+U` | clear the line |
| `Ctrl+W` | delete the last word |
| paste | works; line breaks become spaces |
| `Esc` | close |

With nothing typed, the bar lists your bookmarks and recent pages.

**Choosing the search engine:** Settings → Browsing → *Search engine* offers
DuckDuckGo (the default), Wikipedia and GitHub. For any other engine, put its
search URL in the config file with `%s` where the query goes:

```toml
search_engine = "https://example.com/search?q=%s"
```

Search results are plain pages, so they are readable and clickable like any
other. Engines that need JavaScript to show results will not work.

You can also search straight from the command line: `wr how does tcp work`.

## History, bookmarks and the start page

**History (`v`)** lists the pages you visited, newest first, one line per page
with how long ago. **Bookmarks (`'`)** lists the pages you saved.

| Key (in either panel) | What it does |
|---|---|
| type | filter the list (all words must match the title or address) |
| `↑` `↓`, `PgUp` `PgDn`, `Home` `End` | move |
| `Enter` | open the page |
| `Ctrl+D` | remove the selected entry |
| `Ctrl+X` | (history) clear everything, after asking |
| `Esc` | close |

**To bookmark** the page you are reading, press `*` (press it again to remove
the bookmark). A `★` in the footer shows a bookmarked page.

**The start page** is what `wr` opens when you run it with no address (or when
you press `~`). It lists your bookmarks and recent pages as links. If you would
rather start on a particular page, open it and choose Settings → Browsing →
*Start page*.

History is optional: turn *Remember history* off in Settings and nothing is
recorded. See [Where wr keeps things](#where-wr-keeps-things).

## Searching inside a page

Press `/` and start typing. The page highlights matches **as you type** and
jumps to the first one after where you were.

| Key | What it does |
|---|---|
| `Enter` | accept the search |
| `Esc` | cancel and go back to where you were |
| `n` `N` | next / previous match |
| `Esc` (when not typing) | clear the highlighting |

Search ignores case unless you type a capital letter. The footer shows
`«word» 3/12`. Search works line by line on the text as drawn, so a phrase that
wraps across two lines is not found.

## Selecting and copying

With the mouse:

- **Drag** to select. The text is **copied when you let go**.
- **Double click** selects (and copies) a word; **triple click** a line.
- Drag past the top or bottom edge to keep scrolling while you select.
- `Esc` clears the selection; `y` copies it again.

Copying is smart:

- **Code blocks** copy *without* their frame and the left margin, so what you
  paste is the code.
- **Paragraphs** that were wrapped to fit your window are joined back into the
  single line they were written as.

It copies in two ways at once: with the standard terminal clipboard sequence
(OSC 52, which also works over SSH) and with a system tool when there is one
(`wl-copy`, `xclip`, `xsel`, `pbcopy`; on WSL, PowerShell or `clip.exe`). If
you would rather select with your terminal's own mouse selection, hold `Shift`
while dragging, or turn *Mouse support* off in Settings.

If you want to select first and copy on purpose, turn *Copy on select* off:
the selection stays highlighted and `y` copies it.

## Make it yours

Press `,` (or open the menu with `m` and choose *Settings*). You get a list of
options grouped by topic. Move with `↑` `↓`, change a value with `←` `→` (or
`Enter`), and press `d` to put the selected option back to its default. The
**help for the selected option is shown at the bottom.** Every change applies
immediately and is saved.

| Group | What you can change |
|---|---|
| **Layout** | reading width (or the full terminal), center the column, spacing between blocks (compact / normal / airy), rules under headings, frames around code, how links are shown, braille decoration, accent color |
| **Footer and scrollbar** | the scrollbar, the footer (turn it off for a clean page), its rule, the shortcut hints, the progress display (bar and percent, either, or none), the small notes |
| **Mouse** | mouse support, copy on select, lines per wheel notch |
| **Motion** | the master switch, and separately: smooth scrolling, page reveal, panel animation, loading wave, typewriter notices; plus how quickly scrolling catches up (slow / normal / fast) |
| **Browsing** | search engine, the start page, remember history, how much history to keep, refresh cached pages in the background |
| **Cache** | cache pages, how long to keep them |
| **Other** | Keyboard shortcuts…, Reset all settings… |

Some ideas:

- **Distraction-free reading:** turn off *Footer*, *Scrollbar* and
  *Animations*, set *Reading width* to 80.
- **A quieter look:** *Braille decoration* off, *Heading rules* off, *Spacing*
  compact.
- **Your terminal's colors only:** the accent color is one of your 16 terminal
  colors, so any choice follows your theme. The six presets after them (orange,
  violet, teal, pink, lime, slate) are fixed 256-colors instead: pick one when
  your theme does not have the color you want.
- **Everything instant:** *Animations* off.

All of this lives in one file, `~/.config/wr/config.toml`
(`wr --config` prints its path). It is fully commented and you can edit it by
hand instead; the program and the file edit the same thing. Note that saving
from the menu rewrites the file, so comments you added yourself are not kept.

`wr -L URL` shows links hidden for that session only, without changing your
saved setting.

## Rebinding keys

Open the menu (`m`) and choose *Keyboard shortcuts* (also reachable from
Settings → Other). You see every action with its keys, grouped.

| Key (in this panel) | What it does |
|---|---|
| `Enter` | **rebind**: then press the new key |
| `a` | add another key to the action |
| `x` | unbind the action |
| `d` | restore the action's default keys |
| `R` | restore every default |
| `Esc` | cancel a capture / close |

If the key you press was used by another action, it moves to this one. `Ctrl+C`
(always quit) and `Esc` are reserved. The shortcut hints in the footer show the
keys you actually have. Only the bindings you changed are written to the config
file, under `[keys]`.

## Local files, Markdown and other content

`wr` shows more than web pages:

| Content | How it is shown |
|---|---|
| HTML | the article, formatted |
| Markdown (`.md`, or served as `text/markdown`) | rendered as Markdown (for example `https://raw.githubusercontent.com/.../README.md`) |
| plain text (`.txt`, or served as `text/plain`) | in a code block |
| JSON, XML | in a code block, highlighted |
| images, PDFs, anything else | not shown; you get a notice, and `x` opens it in your real browser |

Local files work the same way: `wr notes.md`, `wr ~/docs/page.html`. Links
between local HTML files work too. `r` reads the file again.

## Using it with other tools

- **Print instead of browse:** `wr URL | less -R` prints the formatted page and
  exits (no interface). `wr --md URL` prints the Markdown, handy for piping into
  other tools. Both always download fresh.
- **Fresh copy:** `wr --fresh URL` ignores the cache once.
- **Over SSH:** it works, and copying uses the OSC 52 route, which travels
  through SSH to your local clipboard (your terminal must allow it).
- **Inside tmux:** it works. For copying to reach your system clipboard through
  tmux, enable `set -g set-clipboard on`. The mouse works as long as your tmux
  passes it through (the default).
- **Version:** `wr --version`.

## Recipes

**Read a long tutorial.** Open it, press `t` to see its sections, `]` and `[` to
move between headings, `/` to find a word, and `*` to bookmark it for later.

**Follow documentation across pages.** Press `f` to label links, type a label,
read, `H` to go back to exactly where you were.

**Look something up.** `o`, type your question, `Enter`; `f` and a label to open
a result; `H` to go back to the results.

**Grab a command from a page.** Drag across the code block; it is copied
without the frame, ready to paste.

**Pick up where you left off.** `v` opens your history; type a few letters to
filter, `Enter` to reopen.

**Read a README.** `wr https://raw.githubusercontent.com/OWNER/REPO/main/README.md`
renders it as Markdown.

## Troubleshooting

**Strange boxes instead of the little dots.** Your font has no braille glyphs.
Turn off *Braille decoration* in Settings, or use a font that has them (most
Nerd Fonts do).

**Copying does not reach the clipboard.** `wr` tries OSC 52 and a system tool.
Make sure your terminal allows OSC 52 (in Alacritty it is on by default for
copying), or install `wl-copy`/`xclip`. In tmux, set `set-clipboard on`. On WSL
it uses PowerShell. A notice says "copied N characters" either way, because the
terminal gives no confirmation.

**I cannot select text with my terminal's mouse.** While mouse support is on,
`wr` handles the mouse: hold `Shift` while dragging to use your terminal's own
selection, or turn *Mouse support* off.

**The page is empty or says very little.** The site probably draws its content
with JavaScript, which `wr` does not run. Try the site's printable or text
version, or `x` to open it in your browser.

**Colors look wrong.** `wr` uses your terminal's 16-color palette, so colors
come from your terminal theme. Pick another accent color in Settings if one is
hard to see, and remember that the presets (orange onwards) keep their color on
every terminal. On a terminal with a light background the palette moves to the
darker half of the 16 colors by itself.

**It starts but looks cramped or odd in a small window.** Make the window
bigger; panels and the footer shrink to fit, but below about 24 columns or 3
rows `wr` shows "too small".

**A saved setting seems ignored.** If the config file has an error, `wr` shows a
notice and uses the defaults. Fix the line or delete the file; Settings will
write a fresh one.

**Reset everything.** Settings → *Reset all settings…* (your key bindings are
kept; menu → Keyboard shortcuts → `R` resets those).

## Where wr keeps things

| Path | What |
|---|---|
| `~/.config/wr/config.toml` | your settings and key bindings |
| `~/.cache/wr/` | the page cache |
| `~/.local/state/wr/history.jsonl` | your history, one JSON line per visit |
| `~/.local/state/wr/bookmarks.json` | your bookmarks |

(The config and state locations follow `XDG_CONFIG_HOME` and `XDG_STATE_HOME`
if you set them.) Folders are private (`0700`) and files readable only by you
(`0600`), except the config file, which is `0644`.

**Privacy:** the cache and the history store the addresses and content of what
you read, on your machine only. To stop it: turn off *Cache pages* and
*Remember history* in Settings. To erase it: menu → *Clear all cache*, the
History panel (`Ctrl+X`), or delete the folders above. `wr --clear-cache`
clears the cache from the command line.
