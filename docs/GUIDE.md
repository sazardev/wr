# wr guide

How to read and browse the web from your terminal with `wr`.

`wr` fetches a page, keeps the article, drops everything else and draws it in
your terminal with your own colors — so it looks like part of your terminal, not
like a website. It has no tabs, no sidebar and no ads: one page at a time, read
with the keyboard. Pages that only exist once their JavaScript runs are rendered
for you when a Chromium or Chrome is installed, and `wr` keeps itself up to date
if you let it.

**The thirty-second version:** run `wr`, press `o`, type an address (or a
question), press `Enter`. `q` quits. Everything else — links, history,
bookmarks, search, settings — is reachable from the menu with `m`, and the
footer always shows the keys you actually have.

If you learn better by doing, jump to [Real use cases](#real-use-cases) and come
back for the details.

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
- [Pages drawn by JavaScript](#pages-drawn-by-javascript)
- [Using it with other tools](#using-it-with-other-tools)
- [Real use cases](#real-use-cases)
- [Updating wr](#updating-wr)
- [Keyboard quick reference](#keyboard-quick-reference)
- [Configuration reference](#configuration-reference)
- [Environment variables](#environment-variables)
- [Troubleshooting](#troubleshooting)
- [Where wr keeps things](#where-wr-keeps-things)

## Start

Install:

```sh
curl -fsSL https://raw.githubusercontent.com/sazardev/wr/main/install.sh | sh
```

Then run it:

```sh
wr                          # the start page
wr https://go.dev/doc/      # an address
wr rust async book          # not an address and not a file: a web search
wr notes.md                 # a local file
```

That is the whole command line. `wr` decides what you mean:

| What you type | What happens |
|---|---|
| an address (`example.com`, `https://example.com/page`, `localhost:8080/app`) | opened; the scheme may be left out (`https://`, or `http://` for localhost, private IPs and `.local`/`.test`/`.lan` names) |
| a file (`./notes.md`, `~/docs/readme.html`, `/etc/hosts`) | opened from disk |
| anything else (`postgres vacuum bloat`) | searched on the web |

Press `q` to quit. `Ctrl+C` always quits too, no matter which panel is open.

**The menu is the map.** Press `m` (or `?`) and you get everything the program
can do, with the key beside each entry: open an address, go back and forward,
history, bookmarks, the section index, reload, settings, the key bindings,
update wr and see what is new, open the page in your real browser, clear the
cache, edit the config file, quit. The title of the menu carries the version
you are running. Press `/` to find an entry by typing (see
[Finding an option](#finding-an-option)). If
you forget a key, this list tells you.

**The first five minutes.** With a page open, try these in order:

| Do this | And you get |
|---|---|
| `j` `k` (or the arrow keys) | line by line, with a smooth glide instead of a jump |
| `space` / `b` | a page down / up |
| `t` | the page's headings: pick one and jump to it |
| `f` | a letter on every link: type the letter to open it |
| `/` then a word | the word highlighted as you type, `n` for the next match |
| `o` | the address bar; with nothing typed it lists bookmarks and recent pages |
| `*` | bookmarks the page (press again to remove) |
| `m` | the menu, where everything else lives |
| `,` | Settings, where every option has a one-line explanation at the bottom |
| `U` | update wr and see what is new in it (see [Updating wr](#updating-wr)) |

## The screen

```
  ⣿ Page title                                                      ⡇   ← the page, with a scrollbar
  ⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀  ⡇
  Text of the page ...                                              ⠿
⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀⣀   ← the rule
 o open  / search  H back  L fwd  f links  t index  m menu  q quit   ⣷⣀⣀⣀  6%   ← the footer
```

- **The page** is the article extracted from the website: menus, footers, ads
  and cookie walls are dropped before you see them. What is left is drawn with
  formatting — headings, lists, tables, code blocks.
- **The scrollbar** on the right shows where you are. It moves smoothly while
  you scroll.
- **The rule** over the page is a braille line: the reader's signature.
- **The footer** has your most useful shortcuts on the left and your reading
  progress on the right. It adapts to the window: a narrow window shows fewer
  shortcuts, and a very short one drops the rule above it.
- **Small notes** appear next to the progress when they matter:

| Note | Meaning |
|---|---|
| `★` | this page is bookmarked |
| `● new version (r)` | the *page* changed on the site while you were reading; `r` reloads it |
| `↑ v0.7.1 (U)` | a newer *wr* exists; `U` shows what is new and updates |
| `↻ updating` / `✓ v0.7.1 · restart` | wr is installing the update / it is installed, restart to use it |
| `«word» 3/12` | a search is active: third match of twelve, `n`/`N` move |

- **Notices** such as `✓ copied 57 characters` replace the shortcuts for about
  a second and disappear the moment you press a key or click.
- Everything is drawn with your terminal's **16 colors**, so wr matches your
  theme. The accent color (panels, keys, marks, bars, and the color the
  headings walk) is changed in Settings, and it has a partner color for the
  secondary highlights (the light end of the footer rule, the search count and
  the focused link).

## Reading a page

| Key | What it does |
|---|---|
| `j` `k` or `↓` `↑` | one line down / up |
| `space` `b` (or `PgDn` `PgUp`) | a page down / up |
| `d` `u` | half a page down / up |
| `g` `G` (or `Home` `End`) | top / bottom |
| `]` `[` | next / previous section (heading) |
| `t` | the **section index**: the page's headings, pick one and jump |
| mouse wheel | scroll (3 lines per notch by default) |

Scrolling is smooth: it glides to where you are going instead of jumping. How
fast it catches up is a setting (*Scroll speed*: slow, normal, fast), and
*Smooth scrolling* off makes it instant.

**Going somewhere in a long page.** `t` opens the section index, which is the
fastest way around: documentation, long articles and API references all get
read top-down with `]` and `[`, or selectively with `t`.

**Code blocks** are highlighted by language and shown in a labeled frame with the
language name on the border. The language comes from the page (`language-xx`
classes, `data-language`, and the conventions of Sphinx and MediaWiki). When a
site ships no language at all — build-time highlighters such as Shiki leave only
colored spans — `wr` infers it, but only when one language clearly wins over the
others among Go, Python, JavaScript, TypeScript, JSON, Bash, SQL, Rust, Java and
C#, and it says so on the frame (`go (guessed)`). A block it is not sure about
stays in plain color, because a wrong color is worse than none. *Guess code
language* off keeps only what the page declares, and *Code frames* off removes
the label and frame if you want the page tighter.

**What is left out.** Besides menus, footers and cookie walls, `wr` drops what
the page itself hides (the `hidden` attribute, `display:none`), permalink anchors
in headings (`¶`, `#`), "skip to content" links, the navigation strips that sit
above a title (breadcrumbs, section labels, `Next:/Up:`), and empty links. A
link that wraps a title and a description (a card) shows the title as the link
and the description under it.

**Width.** By default the text column is 100 columns wide and centered, which is
comfortable to read on a wide terminal. *Reading width* in Settings offers the
usual widths plus *full* for the whole terminal.

## Following links

Three ways, use whichever you like:

1. **Click it.** Links are colored and underlined.
2. **Tab through them.** `Tab` focuses the next link (`Shift+Tab` the previous)
   and the footer shows where it goes. `Enter` opens the focused link, `Esc`
   lets go.
3. **Type a label.** Press `f`: every link on screen gets a short label (`a`,
   `s`, `d`, `f`, ...). Type the label to open that link; `Esc` cancels. With
   your hands on the keyboard this is the fastest way — you do not have to aim.

Notes:

- **How links are displayed** is a setting (*Links*), and it does not change
  whether they work:
  - **footnotes** (the default): a small number after the link text, and the
    address listed at the end of the page.
  - **inline**: the address appears next to the link text.
  - **hidden**: just the text — the cleanest look, and links stay clickable.
- A link to a section of the same page (`#section`) just scrolls there.
- Search engines wrap their results in redirect links so they can count clicks.
  `wr` unwraps them and goes straight to the real target.
- `mailto:`, `javascript:`, `tel:` and similar links are not followed: you get
  a notice that says so (`cannot open this kind of link: …`). Only `http`,
  `https` and local `file` links are opened.
- If a page fails to load you stay on the page you were reading and get a
  notice. Nothing is lost.

## Going back and forward

`wr` remembers the pages of your session like a browser does.

| Key | What it does |
|---|---|
| `H` or `Backspace` or `Alt+←` | back |
| `L` or `Alt+→` | forward |
| your mouse's back / forward buttons | back / forward |

Going back puts you at **exactly the spot you left**, and it is instant because
the page comes from the cache — the whole point of reading a reference: jump
into a link, read the paragraph you needed, `H`, and you are back where you
were. Opening something new from a page you went back to discards the "forward"
pages, as in any browser.

## The address bar and searching the web

Press `o`, type, press `Enter`. The first row always shows what `wr` will do
with what you typed:

| You type | First row says |
|---|---|
| `example.com/page` | `Open example.com/page` |
| `postgres vacuum bloat` | `Search the web for “postgres vacuum bloat”` (with the engine name) |

Below the first row are suggestions: your **bookmarks (`★`)** first, then your
**history**, that match the words you typed. Every word must appear somewhere in
the title or the address, and up to four bookmarks plus eight recent pages are
shown. This is how you retype less: `wr` itself learns what you read.

| Key (in the address bar) | What it does |
|---|---|
| `↑` `↓`, `Tab` / `Shift+Tab` | choose a row |
| `Enter` | open the selected row |
| `Ctrl+U` | clear the line |
| `Ctrl+W` | delete the last word |
| paste | works; line breaks become spaces |
| `Esc` | close |

With nothing typed, the bar lists your bookmarks and recent pages — a quick way
back to something.

**The search engine.** Settings → Browsing → *Search engine* offers DuckDuckGo
(the default), Wikipedia and GitHub. To use anything else, put its search URL in
the config file with `%s` where the query goes:

```toml
search_engine = "https://example.com/search?q=%s"
```

Results are ordinary pages, so they read and behave like any other page. Engines
that need JavaScript to show results will not work.

You can also search straight from the command line: `wr how does tcp work`.

## History, bookmarks and the start page

**History (`v`)** lists the pages you visited, newest first, one line each with
how long ago. **Bookmarks (`'`)** lists the pages you saved. Both work the same
way:

| Key (in either panel) | What it does |
|---|---|
| type | filter the list (all words must match the title or address) |
| `↑` `↓` (or `Ctrl+P` `Ctrl+N`) | move |
| `PgUp` `PgDn` | move eight at a time |
| `Home` `End` | first / last |
| `Enter` | open the page |
| `Ctrl+D` | remove the selected entry |
| `Ctrl+X` | (history only) clear everything, after asking |
| `Esc` | close |

**To bookmark** the page you are reading press `*`; press it again to remove it.
A bookmarked page shows `★` in the footer notes.

**The start page** is what `wr` opens with no address (or when you press `~`).
It is a real page: your bookmarks (up to 12) and recent pages (up to 15) as
links, so you can open them with the mouse or with `f`. If you would rather
start on a particular page, open that page and choose Settings → Browsing →
*Start page* (on the start page it goes back to the start page).

History is optional: turn *Remember history* off and nothing is recorded, and
the panels say so instead of guessing. See [Where wr keeps things](#where-wr-keeps-things).

## Searching inside a page

Press `/` and start typing. The page highlights matches **as you type** and
jumps to the first one after where you were.

| Key | What it does |
|---|---|
| `Enter` | accept the search |
| `Esc` | cancel and go back to where you were |
| `n` `N` | next / previous match |
| `Esc` (when not typing) | clear the highlighting |

Search ignores case unless you type a capital letter (smart case, like vim: type
`Fprintf` to match only that). The footer shows `«word» 3/12`.

Search works line by line on the text as it is drawn, so a phrase that wraps
across two lines of your window is not found. To fix that, widen the window or
use a single word.

## Selecting and copying

With the mouse:

- **Drag** to select. The text is **copied when you let go** — there is nothing
  else to press.
- **Double click** selects (and copies) a word; **triple click** a line.
- Drag past the top or bottom edge to keep scrolling while you select.
- `Esc` clears the selection; `y` copies it again.

Copying is smart about the two things terminal readers usually get wrong:

- **Code blocks** copy *without* their frame and left margin, so what you paste
  is the code, ready to run.
- **Paragraphs** that were wrapped to fit your window are joined back into the
  single line they were written as.

A notice says how much was copied (`✓ copied 57 characters`).

Copying happens over two routes at once: the standard terminal clipboard
sequence (OSC 52, which also works over SSH) and a system tool when there is one
(`wl-copy`, `xclip`, `xsel`, `pbcopy`; on WSL, PowerShell or `clip.exe`). Your
terminal gives no confirmation of its own, which is why `wr` shows the notice.

If you would rather select with your terminal's own mouse selection, hold `Shift`
while dragging, or turn *Mouse support* off in Settings. If you prefer to select
first and copy on purpose, turn *Copy on select* off: the selection stays
highlighted and `y` copies it.

## Make it yours

Press `,` (or open the menu with `m` and choose *Settings*). You get a list of
options grouped by topic, the value of the selected one on the right and its
**help at the bottom**.

| Key (in Settings) | What it does |
|---|---|
| `↑` `↓` (or `k` `j`) | move |
| `PgUp` `PgDn` | move six at a time |
| `Home` `End` (or `g` `G`) | first / last |
| `←` `→`, `Enter` or `Space` | change the value |
| `d` or `Backspace` | put this option back to its default |
| `Esc` or `q` | close |

Every change applies immediately and is saved to the config file.

### Finding an option

The menu, Settings, the keyboard shortcuts and the section index (`t`) all have
the same filter. Press **`/`** and type: only the entries that mention every word
you typed stay, in any order and ignoring case. Settings looks at the name, the
group and the help text (`cache`, `braille`, `scroll`, `update`), the menu at the
name, shortcuts at the action and its keys, the index at the heading.

| Key | What it does |
|---|---|
| `/` | start filtering (again, to edit what you typed) |
| *typing* | narrows the list; `↑` `↓` move among the matches, `Backspace` deletes |
| `Enter` | in the menu and the index: do it (open the entry, jump to the section). In Settings and shortcuts: keep the results so you can change the value you found, then `←` `→` or `Enter` act on it |
| `Esc` | clears the filter (a second `Esc` closes the panel) |

The footer shows the keys that apply at each moment. Everything you type goes
into the filter, so `q`, `j` or a digit are letters there, not commands. When you
are changing a key binding, `/` is a key like any other and can be bound.

| Group | Options (exact panel names) |
|---|---|
| **Layout** | Reading width, Center the column, Spacing, Heading rules, Code frames, Guess code language, Links, Braille decoration, Accent color |
| **Footer and scrollbar** | Scrollbar, Footer, Footer rule, Shortcut hints, Reading progress, Footer notes |
| **Mouse** | Mouse support, Copy on select, Wheel lines |
| **Motion** | Animations (master switch), Smooth scrolling, Scroll speed, Page reveal, Panel animation, Loading wave, Typewriter notices |
| **Browsing** | Search engine, Start page, Render JavaScript pages, Check for updates, Show what's new, Remember history, History size, Refresh in the background |
| **Cache** | Cache pages, Keep pages for |
| **Other** | Keyboard shortcuts…, Reset all settings… |

Three ready-made setups, straight into `~/.config/wr/config.toml`:

```toml
# Distraction-free: just the text, centered, no chrome but the scrollbar
width = 80
center = true
spacing = 1
footer = false
footer_rule = false
scrollbar = false
animations = false
braille = false
heading_rules = false
```

```toml
# A terminal-native look: your 16 colors only, links inline, quiet motion
accent = "green"
links = "inline"
anim_reveal = false
anim_panels = false
anim_notices = false
scroll_speed = "fast"
wheel_lines = 5
```

```toml
# Minimal disk footprint: cache off, history off
cache = false
history = false
```

```toml
# Nothing but the pages you open: no update check, no scripts run on your behalf
update_check = false
render_js = false
```

```toml
# Know what changed: the notes window opens by itself when wr has an update
update_check = true
whats_new = true
```

The whole configuration lives in one file, `~/.config/wr/config.toml`. `wr --config`
creates it if missing and prints its path. You can edit it by hand with anything
— including from inside `wr`: the menu has *Edit the config file…*, which opens
`$VISUAL` or `$EDITOR` (default `vi`) and reloads the settings when you close
it. The panel and the file edit the same thing. One caveat: saving from the
Settings panel rewrites the whole file, so comments you added by hand are not
kept.

`wr -L URL` hides links for that session only, without changing your saved
setting; `wr --fresh URL` ignores the cache once.

## Rebinding keys

Open the menu (`m`) and choose *Keyboard shortcuts* (also Settings → Other →
*Keyboard shortcuts…*). You see every action with its keys, grouped by topic.

| Key (in this panel) | What it does |
|---|---|
| `↑` `↓` (or `k` `j`) | move |
| `Enter`, `→` or `l` | **rebind**: then press the new key |
| `a` | add another key to the action |
| `x` | unbind the action entirely |
| `d` or `Backspace` | restore this action's default keys |
| `R` | restore every default |
| `Esc` | cancel a capture / close |

If the key you press was used by another action, it moves to this one — you
never end up with two actions on one key. `Ctrl+C` (always quit) and `Esc` are
reserved. The footer hints show the keys you actually have, and only the
bindings you changed are written to the config file, under `[keys]`:

```toml
[keys]
half_down = ["ctrl+d", "D"]
quit = ["q", "Z"]
```

## Local files, Markdown and other content

`wr` shows more than web pages:

| Content | How it is shown |
|---|---|
| HTML | the article, formatted |
| Markdown (`.md`, or served as `text/markdown`) | rendered as Markdown |
| plain text (`.txt`, or served as `text/plain`) | in a code block |
| JSON, XML | in a code block, highlighted |
| images, PDFs, anything else | not shown: you get a notice, and `x` opens it in your real browser |

Local files work the same way as remote ones: `wr notes.md`, `wr ~/docs/page.html`,
`wr ./README.md`, `wr file:///etc/hosts`. Links between local HTML files work,
`f` labels them like any other link, and `r` reads the file again from disk.

## Pages drawn by JavaScript

Some sites send an almost empty page and draw everything with JavaScript — the
single-page apps behind many documentation sites (Vite, React, Vue, Angular…).
`wr` does not run JavaScript itself, so it deals with them in layers, and it
only starts down this road for a page that came out nearly empty: a normal page
never pays for any of it.

**1. It recognises the page.** An empty (or "Loading…") mount point such as
`#root`, `#app` or `<app-root>`, a `<noscript>` asking for JavaScript, a module
script on a bare page, or a page with scripts and hardly any text.

**2. It looks for a text version.** If the page advertises one
(`<link rel="alternate" type="text/markdown">`), `wr` reads that.

**3. It renders the page, if it can.** With a Chromium or Chrome installed
(`chromium`, `chromium-browser`, `google-chrome`, `brave-browser`,
`microsoft-edge`… on your `PATH`, or the one named in `WR_BROWSER`), `wr` loads
the page in a headless instance and reads the result like any other page. The
loading screen and the footer say *rendering with the browser…*; it takes a few
seconds and is cut off after 30. The browser runs with its sandbox on and a
throwaway profile that is deleted afterwards, and the rendered page is cached
like any other.

**4. Otherwise it tells you, and shows what it has.** The page's own summary
(its `<noscript>` text, description and JSON-LD) appears under a notice that says
what to do: install a browser, turn `render_js` back on, or open the page with
`x`. This version is not cached, so the page renders properly once a browser
exists.

What this cannot do: pages behind a login, sites that turn away automated
browsers (the "Just a moment…" walls), and content that only appears after you
click something. `x` opens the page in your real browser, and `wr --md URL`
shows the Markdown `wr` ended up with.

**It is your call.** Rendering runs the site's own scripts, and whatever network
requests they make, inside that headless browser. If you would rather not load a
site's scripts at all, turn it off: *Render JavaScript pages* in Settings,
`render_js = false`, or `WR_BROWSER=/nonexistent`. Pages drawn by JavaScript then
show their summary and the notice.

## Using it with other tools

- **Print instead of browse:** `wr URL | less -R` prints the formatted page and
  exits — no interface, no interactivity, ideal for a quick look or a monitor.
  Width comes from *Reading width* or the `COLUMNS` variable (`COLUMNS=60 wr URL | less -R`). With `NO_COLOR` set (or `TERM=dumb`) the printed render has no colors at all; the interactive reader always uses them.
- **Pipes of Markdown:** `wr --md URL` prints the Markdown, uncolored. Pipe it
  into whatever you like:

  ```sh
  wr --md https://go.dev/doc/effective_go | grep -n "gofmt"
  wr --md https://raw.githubusercontent.com/sazardev/wr/main/README.md > wr-readme.md
  ```

- **Add `-L`** to any of these to drop the link addresses (`--md -L URL`).
- **Fresh copy:** `wr --fresh URL` ignores the cache once.
- **Over SSH:** it works, and copying uses the OSC 52 route, which travels
  through SSH to your local clipboard (your terminal must allow it).
- **Inside tmux:** it works. For copying to reach your system clipboard through
  tmux, enable `set -g set-clipboard on`. The mouse works as long as your tmux
  passes it through (the default).
- **Version and updates:** `wr --version` prints the version, `wr --update`
  looks for a newer release and installs it. **Help:** `wr --help`.

The pipe modes (`| less -R` and `--md`) always download the page again, so what
you print is what the site says now; they also refresh the cache for next time.

## Real use cases

Ten situations you actually meet, with the exact keys. Nothing here needs a
setting you do not already have.

### Read one long article

An essay, a blog post, a paper — you want the text and nothing else.

```sh
wr https://blog.example/posts/the-case-for-boring-tools
```

```text
t              sections: jump straight to the part you need
]   [          read it top-down, heading by heading
/   word       find a term; n jumps to the next
*              bookmark it: `~` brings it back tomorrow
d   u          half a page at a time when the text is dense
```

If the page has a paywall JavaScript wall, `x` opens it in your browser.

### Follow documentation across pages

Library docs are the reason `H` exists: jump in, read one paragraph, jump back.

```sh
wr https://go.dev/doc/effective_go
```

```text
f              label every link on the page
s              open the one labelled "s" — say, "Formatting"
…read…
H              back on this page, at exactly this line
f              next link
```

With ten pages open in a row, `v` lists them all — you can jump between them
with the arrow keys and `Enter` instead of walking back one by one.

### Research a question end to end

```sh
wr postgres vacuum bloat
```

```text
(three or four results, as a normal page)
f    s         open the first result
read, and if the page has what you need:
*              bookmark it
H              back to the results
f    d         open the next one
```

Or stay on the results page and use `/` to find the word you came for, then `f`
to open the link that mentions it.

### Copy commands from a tutorial

```sh
wr https://example.com/tutorials/deploy-with-docker
```

```text
drag over the code block, or double-click a line
   the copy drops the frame and the left margin: paste and run
y              copy again if you need it twice
```

For a command inside a paragraph, copy the line the same way: wrapped paragraphs
are joined back into one line, so a long command pastes as one line.

### Read a project README

```sh
wr https://github.com/sazardev/wr            # the rendered page
wr https://raw.githubusercontent.com/sazardev/wr/main/README.md
```

The second form is the Markdown itself, ideal for READMEs: headings, tables and
code render exactly as on GitHub, with no navigation around them.

### Read your own notes offline

```sh
wr ~/notes/ideas.md
wr ./docs/deploy.html
```

Notes are local pages: `f` labels the links between them, `t` gives you the
headings, `/` finds a word. A folder of Markdown files becomes a small
knowledge base you can read on a plane.

### A morning scan with the start page

Bookmark the handful of sites you read daily (`*` on each one), then every
morning just run `wr`: the start page lists them as links. `f` and one letter
opens the first; `H` brings you back to the list. `v` shows what you read
yesterday if you want to continue where you left off.

### Read on a remote machine over SSH

```sh
ssh build-server
wr https://go.dev/doc/
```

Everything works, including copying: the text goes to the clipboard of the
*machine you are sitting at* through OSC 52, as long as your terminal allows it
(most do; Alacritty and kitty out of the box). Drag-select is the fastest way
here, since the remote machine has no clipboard of its own worth using.

### Feed a page to your editor or a script

```sh
wr --md https://go.dev/doc/effective_go | grep -n "interface"
wr --md URL | pandoc -f gfm -t html
COLUMNS=60 wr https://example.com/changelog | less -R
```

Use `--md` for the Markdown, the plain pipe for a formatted render, `-L` to drop
link addresses, `--fresh` when you know the cached copy is stale.

## Updating wr

The version you are running is in the title of the menu (`m`), on the start page
and in `wr --version`; the footer stays free of it. A build made between releases
reads `v0.7.1-dev`.

**wr looks for a newer release by itself.** In the reader, a few seconds after
it starts, it asks GitHub for the latest release, at most once every 12 hours
(one request to `api.github.com`, nothing about you is sent; the answer is
remembered in `~/.local/state/wr/update.json`). If there is a newer one you get a
notice once, and the footer keeps a reminder: `↑ v0.6.1 (U)`. Nothing is
downloaded until you ask, and the check never runs for `--md`, pipes or the MCP
server. Turn it off with *Check for updates* in Settings, `update_check = false`
or the environment variable `WR_NO_UPDATE_CHECK=1`.

**To update, press `U`** (or menu → *Update wr / what's new…*). The window shows
what the new version brings and asks for `enter`:

```
╭─ wr v0.6.1 is available ─────────────────────────────────╮
│ You have v0.6.0.                                         │
│                                                          │
│ Released 2026-10-20                                      │
│                                                          │
│ Features                                                 │
│ • read script-drawn pages, guess code languages …        │
│                                                          │
│ enter update now  esc later  o release page              │
╰──────────────────────────────────────────────────────────╯
```

`wr` downloads the archive for your system from the project's GitHub release,
checks its SHA-256 against the release's `checksums.txt`, runs the new binary to
make sure it works and is the version it claims to be, and only then replaces
the running one (atomically, so a failure leaves your current wr untouched).
You can keep reading while it works; when it finishes, restart `wr` to use it.
If the folder is not writable (a system-wide install), it says so and points to
`install.sh` or `sudo wr --update`.

**From the command line:** `wr --update` does the same without the reader,
printing the release notes first.

**The "what's new" window is off by default.** Turn on *Show what's new*
(`whats_new = true`) and the window opens by itself, once per version, when an
update is found (never while you are typing a search or an address), and again
on the first launch after you update, with the notes of the version you are now
on. `U` opens it whenever you like, whatever the setting.

Builds that carry no release version (a plain `go build` between tags reads
`dev`) never look for updates: there is nothing to compare.

## Keyboard quick reference

The default bindings, as shown by the menu → *Keyboard shortcuts*. Every one of
them can be changed (see [Rebinding keys](#rebinding-keys)); the footer always
shows the keys you actually have.

**Browsing**

| Key | Action |
|---|---|
| `o` | open an address or search the web |
| `H` `Backspace` `Alt+←` | back |
| `L` `Alt+→` | forward |
| `r` | reload the page |
| `~` | the start page |
| `v` | history |
| `'` | bookmarks |
| `*` | bookmark / remove this page |
| `x` | open in your browser |

**Scrolling**

| Key | Action |
|---|---|
| `j` `↓` | line down |
| `k` `↑` | line up |
| `space` `PgDn` `Ctrl+F` | page down |
| `b` `PgUp` `Ctrl+B` | page up |
| `d` `Ctrl+D` | half page down |
| `u` `Ctrl+U` | half page up |
| `g` `Home` | top |
| `G` `End` | bottom |

**Reading**

| Key | Action |
|---|---|
| `]` | next section (heading) |
| `[` | previous section |
| `t` | the section index |
| `/` | search in the page |
| `n` / `N` | next / previous match |
| `y` | copy the selection |

**Links**

| Key | Action |
|---|---|
| `f` | follow a link (hint labels) |
| `Tab` / `Shift+Tab` | next / previous link |
| `Enter` | open the focused link |

**Interface**

| Key | Action |
|---|---|
| `m` or `?` | the menu |
| `,` | settings |
| `U` | update wr / what's new |
| `q` | quit |

Reserved everywhere: `Ctrl+C` (always quits) and `Esc` (cancel).

## Configuration reference

Every key of `~/.config/wr/config.toml`, with its default. Anything missing uses
its default, a broken file falls back to the defaults with a notice, and most of
this can be changed live from Settings.

| Key | Default | Values | Meaning |
|---|---|---|---|
| `width` | `100` | `0` or 40–400 | max width of the reading column; `0` = full width |
| `center` | `true` | bool | center the column in the terminal |
| `spacing` | `1` | `0` `1` `2` | blank lines between blocks: compact, normal, airy |
| `heading_rules` | `true` | bool | a rule under level 1 and 2 headings |
| `code_frame` | `true` | bool | the label and frame around code blocks |
| `guess_code_lang` | `true` | bool | infer the language of unlabeled code when wr is sure (shown as `(guessed)`) |
| `links` | `"footnotes"` | `footnotes` `inline` `hidden` | how link addresses are shown |
| `braille` | `true` | bool | braille decoration (marks, bars, spinner) |
| `accent` | `"blue"` | 8 ANSI colors + 6 presets | the color of the UI chrome |
| `scrollbar` | `true` | bool | the scrollbar on the right |
| `footer` | `true` | bool | the footer row |
| `footer_rule` | `true` | bool | the thin rule above the footer |
| `footer_hints` | `true` | bool | the shortcut hints on the left of the footer |
| `footer_progress` | `"both"` | `both` `percent` `bar` `off` | what the footer shows on the right |
| `footer_chips` | `true` | bool | the small notes: bookmark star, update available, search count |
| `mouse` | `true` | bool | wheel scrolling, clicking links, drag to select |
| `copy_on_select` | `true` | bool | copy as soon as the mouse is released |
| `wheel_lines` | `3` | 1–20 | lines per wheel notch |
| `animations` | `true` | bool | master switch for every animation |
| `anim_scroll` | `true` | bool | smooth (spring) scrolling |
| `anim_reveal` | `true` | bool | the page wiping in when it loads |
| `anim_panels` | `true` | bool | panels opening like a shutter |
| `anim_loading` | `true` | bool | the braille wave while a page downloads |
| `anim_notices` | `true` | bool | notices typing themselves out |
| `scroll_speed` | `"normal"` | `slow` `normal` `fast` | how quickly scrolling catches up |
| `search_engine` | `"duckduckgo"` | `duckduckgo` `wikipedia` `github`, or a URL with `%s` | where plain words go |
| `homepage` | `""` | an address or a file | what `wr` opens with no address; `""` = the start page |
| `render_js` | `true` | bool | render pages drawn by JavaScript with an installed Chromium/Chrome |
| `update_check` | `true` | bool | look for a newer wr in the background (at most every 12 hours) |
| `whats_new` | `false` | bool | show the release notes in a small window when there is an update, and after you update |
| `history` | `true` | bool | remember the pages you visit |
| `history_limit` | `1000` | 50–20000 | how many visits to keep |
| `background_refresh` | `true` | bool | refresh a cached page while you read it, and tell you if it changed |
| `cache` | `true` | bool | keep pages on disk |
| `cache_days` | `30` | 1–3650 | days before a cached page is forgotten |
| `[keys]` | — | `action = ["key", …]` | only the bindings that differ from the defaults |

The eight ANSI accent colors (`blue`, `cyan`, `green`, `magenta`, `yellow`,
`red`, `gray`, `white`) are your terminal's own colors, so they follow its
theme. The six presets (`orange`, `violet`, `teal`, `pink`, `lime`, `slate`)
are fixed 256-colors that look the same everywhere — pick one when your theme
lacks the color you want.

## Environment variables

| Variable | What it does |
|---|---|
| `NO_COLOR`, `TERM=dumb` | the printed render (`wr URL \| less`) has no colors; the interactive reader always uses them |
| `COLUMNS` | width of the printed render (`COLUMNS=60 wr URL \| less -R`) |
| `WR_BROWSER` | the Chromium or Chrome used to render pages drawn by JavaScript (a name or a path; `/nonexistent` means none) |
| `WR_NO_UPDATE_CHECK` | any value turns off the background check for a newer wr |
| `VISUAL`, `EDITOR` | the editor opened by *Edit the config file…* (default `vi`) |
| `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME` | where wr keeps its config, its state (history, bookmarks, update memory) and its cache |

## Troubleshooting

**Strange boxes instead of the little dots.** Your font has no braille glyphs.
Turn off *Braille decoration* in Settings, or use a font that has them (most
Nerd Fonts do).

**Copying does not reach the clipboard.** `wr` tries OSC 52 and a system tool.
Make sure your terminal allows OSC 52 (in Alacritty it is on by default for
copying), or install `wl-copy`/`xclip`. In tmux, `set -g set-clipboard on`. On
WSL it uses PowerShell. The notice says "copied N characters" either way,
because the terminal gives no confirmation of its own.

**I cannot select text with my terminal's mouse.** While mouse support is on,
`wr` handles the mouse: hold `Shift` while dragging to use your terminal's own
selection, or turn *Mouse support* off.

**The page is empty or says very little.** The site draws its content with
JavaScript. If a Chromium or Chrome is installed `wr` renders it by itself; if
not, install one, or press `x` to read the page in your browser. See
[Pages drawn by JavaScript](#pages-drawn-by-javascript).

**"HTTP 403 (“…”)" or "HTTP 401".** The text in quotes is the title of the page
that answered, which tells you who said no: a site that turns away non-browser
clients ("Just a moment…" is Cloudflare), or your own network (a company proxy
answers with its own "access restricted" page). `x` opens the address in your
browser, where a login or a human check can be passed.

**"could not download: timed out" or "connection refused".** The site is slow or
unreachable, or (for `localhost:3000`) the server is not running. A reset or a
timeout is retried once before it is reported.

**"Install Chromium or Chrome and wr renders such pages by itself".** That is
the notice of a page drawn by JavaScript with no browser to render it. If you
have one, check it is on your `PATH` (`WR_BROWSER` names it explicitly) and that
`render_js` is on.

**Code has no color.** Colors come from the language the page declares. A page
that declares none gets a guess only when `wr` is sure of it (the frame then says
`(guessed)`); anything else stays plain. Languages without a lexer (`mermaid`)
are never colored.

**The update did not install.** The message says why. "cannot write to …" means
the folder holding `wr` is not yours (a system-wide install): run
`sudo wr --update` or use `install.sh`. A checksum mismatch means the download
was corrupt and nothing was changed; try again. `wr --update` prints every step.
A build made with a plain `go build` between releases carries no version to
compare, so it never looks for updates.

**The update notice never appears.** The check runs at most every 12 hours, in
the reader only, and is off with `update_check = false` or `WR_NO_UPDATE_CHECK`.
`U` → `c` checks right now.

**"cannot open this kind of link: …".** The link is not a page: `mailto:`,
`javascript:`, `tel:` and friends are not followed on purpose. Copy the address
by hand if you need it.

**Colors look wrong.** `wr` uses your terminal's 16-color palette, so the colors
come from your terminal theme. Pick another *Accent color* in Settings if one is
hard to see — and remember that the presets (orange onwards) keep their color on
every terminal. On a terminal with a light background the palette moves to the
darker half of the 16 colors by itself.

**It starts but looks cramped, or says "too small".** Panels and the footer
shrink to fit, but below about 24 columns or 3 rows there is nothing to draw.
Make the window bigger.

**A saved setting seems ignored.** If the config file has an error, `wr` shows a
notice and uses the defaults. Fix the line or delete the file; Settings writes a
fresh one. `wr --config` prints (and creates) the file.

**A page I already read loads slowly.** It was either not cached (an image, a
local file, or the cache is off) or older than *Keep pages for*. `r` reloads it
and refreshes the cache.

**Links on the page do nothing when I click them.** With *Links* set to
*footnotes* the link text stays clickable and every address is also listed at
the end of the page, clickable there. If nothing responds at all, check that
*Mouse support* is on.

**Search cannot find a phrase I can see on screen.** Search works line by line,
so a phrase wrapped across two lines of your window is not found — search for
one word instead.

**Reset everything.** Settings → *Reset all settings…* keeps your key bindings;
to reset those too, menu → *Keyboard shortcuts* → `R`.

## Where wr keeps things

| Path | What |
|---|---|
| `~/.config/wr/config.toml` | your settings and key bindings |
| `~/.cache/wr/` | the page cache |
| `~/.local/state/wr/history.jsonl` | your history, one JSON line per visit |
| `~/.local/state/wr/bookmarks.json` | your bookmarks |
| `~/.local/state/wr/update.json` | the newest release seen, and which version was announced |

(The config and state locations follow `XDG_CONFIG_HOME` and `XDG_STATE_HOME`
if you set them.) Folders are private (`0700`) and files readable only by you
(`0600`), except the config file, which is `0644`.

**Privacy:** the cache and the history store the addresses and the content of
what you read, on your machine only. To stop it: turn off *Cache pages* and
*Remember history* in Settings. To erase what is already there: menu → *Clear
all cache*, the History panel (`Ctrl+X`), or delete the folders above.
`wr --clear-cache` clears the cache from the command line.

**What wr sends over the network:** the pages you open (and the web search you
type, to the engine you chose), at most one request every 12 hours to
`api.github.com` to look for a newer release (off with *Check for updates*), and,
for a page drawn by JavaScript, whatever that page's own scripts request while a
headless browser renders it (off with *Render JavaScript pages*). Nothing else,
and nothing about you beyond the `User-Agent` and the language of your locale.
