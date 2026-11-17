# The web edition: wr's TUI in a browser

This site is not a marketing page with a documentation section. It is the
reader itself: one fixed terminal frame, the same glyphs, the same 16 ANSI
colors and the same keys. The content changes; the frame does not.

The previous blueprint design is gone. There are no cards, no hero, no
gradients, no shadows, no rounded corners, no sans-serif prose and no separate
site header. What you see is what `wr` draws.

- [Rules](#rules)
- [The frame](#the-frame)
- [The palette](#the-palette)
- [Type and glyphs](#type-and-glyphs)
- [Content](#content)
- [Panels](#panels)
- [Keyboard](#keyboard)
- [Media](#media)
- [Responsive and print](#responsive-and-print)

## Rules

1. **One frame.** The page is a terminal: a title bar with `⣿` and `⡇`, a
   braille rule, a scrollable body with a braille scrollbar, another rule and a
   footer with the keys and the reading position. The frame never scrolls away.
2. **The program is the reference.** Glyphs, headings, rules, code frames,
   bullets, quotes and the progress bar are the shapes `wr` itself draws. If a
   shape does not exist in the reader, it does not exist here.
3. **Monospace only.** Everything, prose included, uses the same monospace
   stack. There is no second typeface and no reading font.
4. **The terminal's colors.** The palette is the 16 ANSI colors. The accent
   setting changes headings, links, the scrollbar, selections and the progress
   bar; the rest stays the terminal's.
5. **No motion.** The TUI redraws; it does not fade, rise or slide. The only
   animation is the caret the browser draws in an input.
6. **The keyboard first.** Every destination and every panel has a key. The
   mouse works, but nothing requires it.
7. **The content is the same content.** Pages are still built from
   `docs/GUIDE.md`, `docs/DESIGN.md`, `docs/BENCHMARKS.md`,
   `docs/ARCHITECTURE.md`, `CHANGELOG.md` and the generated reference. The
   shell changed; the files did not.

## The frame

`web/src/layouts/Base.astro` renders the shell and `web/src/styles/tui.css`
draws it:

| Part | What it is |
|---|---|
| `.wr-head` | `⣿` · the page (`guide / start`) · `⡇` |
| `.wr-rule` | three spans of `⣀` in the accent's dim, normal and partner colors |
| `.wr-stage` | the body and the one-column braille scrollbar |
| `.wr-body` | the only scroll container; native scrollbars are hidden |
| `.wr-scroll` | `⡇` track and a smooth braille thumb, redrawn on scroll |
| `.wr-foot` | the keys on the left, the search count and progress on the right |

The body is 100 `dvh` tall and the grid does the rest. The reading column is
`100ch` by default; the Settings panel changes it to 72, 100, 120 or full.
`Center text` centers or left-aligns that column.

## The palette

The base is Tomorrow Night, written as the first 16 ANSI colors in
`web/src/styles/tui.css`. The accent is one of the same names the program's
Settings panel cycles: `blue`, `cyan`, `green`, `magenta`, `yellow`, `red`,
`gray`, `white`, `orange`, `violet`, `teal`, `pink`, `lime`, `slate`.

Shiki runs in `css-variables` mode, so code uses the same terminal colors:
keywords are magenta, strings green, comments dim, functions blue, constants
and links cyan. There is no second theme and no per-language override.

## Type and glyphs

The monospace stack is Geist Mono, then the system's terminal monospace. The
reading size is 14px (`13px` under 720px) with a 1.55 line height.

| Element | Glyph |
|---|---|
| title, h1 | `⣿` and a `⣀` rule |
| h2 | `⣶` and a `⠤` rule |
| h3–h6 | `⠶`, `⠤`, `⠒`, `⠂` |
| bullets | `•`, `◦`, `▪` |
| quote | `▎` |
| code frame | `╭─ lang`, `│`, `╰─` |
| scrollbar | `⡇` track, braille dots for the thumb |
| progress | `⣀`→`⣿` fill, `⣀` empty |
| error | `✗` |

With braille off in Settings the headings switch to `█ ◢ ▎ ▏ ·`, the rules to
`━`/`─`, and the scrollbar and progress bar to block characters.

## Content

The rendered Markdown is the reader's Markdown: h2/h3 with their glyph and
rule, paragraphs at the same rhythm, lists with the console bullets, tables
with terminal borders, `kbd` as key caps and code in a labelled frame. Links
are underlined with dots and invert to the accent on hover, like a focused
link. Search highlights every match in the accent and the current match in the
partner color.

The start page (`/`) is the reader's start page: `wr`, the install line, the
documents as bookmarks and the release you are reading as recent. There is no
hero and no product window.

## Panels

Every panel is a box drawn over the frame, with a title cut into the top
border, a scrollable body and a help line at the bottom:

| Key | Panel | What it does |
|---|---|---|
| `m` | Menu | Navigate, this page, wr, tools. `j`/`k` choose, `/` filters. |
| `t` | Section index | Every heading of the page; `enter` jumps. |
| `?` | Keyboard shortcuts | The web key map, grouped like the program's panel. |
| `,` | Settings | Accent, width, spacing, heading rules, code frames, braille, centering. |
| `o` | Open address or search | The same page index as search, with the menu's commands. |
| `/` | Search | A footer prompt, not a panel: matches are highlighted in place. |

The menu is server-rendered and works with JavaScript off as a list of links.
The rest of the interaction is progressive: without JavaScript the pages and
links still work.

## Keyboard

| Key | Does |
|---|---|
| `j` `k` / `↓` `↑` | line down / up |
| `d` `u` | half page down / up |
| `space` `b` | page down / up |
| `g` `G` | top / bottom |
| `/` | search this page |
| `n` `N` | next / previous match |
| `t` | section index |
| `f` | label every link; type the label |
| `o` | open address or search |
| `H` `L` | back / forward |
| `r` | reload |
| `q` | start page |
| `[` `]` | previous / next page of a document |
| `y` | copy the selection, or the address |
| `Y` | copy the code block on screen |
| `Ctrl+Y` | copy the page as Markdown |
| `U` | releases |
| `m` `?` `,` | menu, help, settings |
| `esc` | close a panel or clear a search |

The keys never fire while a field has focus. The full, generated list is at
[Shortcuts](/docs/reference/shortcuts/).


## Media

Images and video are **off by default**. A page that carries media prints the
same dim line the reader prints when a protocol is unavailable:

```
[image: the reader, captured] open ↗
[video: the reader, recorded] open ↗
```

Open Settings (`,`) and turn on **Images** or **Videos** under *Media*. Images
are fetched lazily when they come near the viewport, decoded off the main
thread, and stored in the browser's media cache; **Media cache** in the same
panel shows the count and size and clears it. Videos stream with
`preload="metadata"`; their posters use the same image cache.

The image fit (contain / cover), column width, video autoplay, loop, mute and
native controls are all settings, persisted with the rest. Autoplay is forced
to muted because browsers require it.

<img src="/assets/wr-preview.png" alt="The reader, captured in the terminal" width="1800" height="1170">

<video poster="/media/reader-demo-poster.png" title="The reader, recorded" width="960" height="540">
  <source src="/media/reader-demo.webm" type="video/webm">
  <source src="/media/reader-demo.mp4" type="video/mp4">
</video>

## Responsive and print

Under 720px the body uses one column of margin, the hints drop their less
important entries and the panels take 96vw. Under 520px the progress bar hides
and the title shrinks. The shell still fills the viewport; the content remains
the same.

Printing drops the shell entirely: white paper, black text, the article only,
with code frames as thin rules. The screen is the TUI; the printed page is the
article.
