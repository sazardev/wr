# wr design system

The design language of the documentation site (the Astro app in `web/`) and of the
recordings in `media/`: a **blueprint** drawn in the terminal's own vocabulary —
monochrome, ruled, sharp, keyboard-first. Version: **DS-2.0** (2026-10-11).

- [Rules](#rules)
- [Tokens: the ten colors](#tokens-the-ten-colors)
- [Themes](#themes)
- [Type](#type)
- [Layout](#layout)
- [Components](#components)
- [Motion](#motion)
- [Keyboard](#keyboard)
- [The recordings](#the-recordings)

## Rules

1. **Monochrome.** Each palette is one hue: a ramp of it for the background, the
   panels and the lines, and the same hue at full strength as the accent. There
   is no second color, no status red or green; state is shown with position,
   weight and the accent.
2. **Flat and sharp.** No `border-radius`, no `box-shadow`, no `text-shadow`, no
   gradient (`linear-gradient` appears in no file), no blur. One translucency is
   allowed on purpose: the scrim behind a dialog, whose job is to dim the page.
3. **Lines are the structure.** Blocks are separated by a `1px` rule. The page is
   a *frame* between two vertical rails; every band of it (a *sheet*) starts with
   a crosshair `+` on each rail, like the registration marks of a drawing; the
   margins outside the frame are hatched; the hero and the closing band sit on a
   blueprint grid (minor line every 32px, major every 160px).
4. **Two typefaces, one job each.** Geist Mono is the interface: headings, labels,
   keys, code, the status line. Geist is for reading: the lead paragraphs and
   the prose of the guide. Both are self-hosted variable fonts; the Latin subsets
   are preloaded.
5. **The program is the reference.** The status line, the key caps, the
   `[ 01 ]` section labels, the labeled code frames and the `⣿` mark are the
   shapes `wr` draws, so the docs read like a page opened in the reader.
6. **The keyboard first.** Everything has a key (see [Keyboard](#keyboard)); the
   mouse is never required and the keys are discoverable from the page itself.
7. **Adaptive, not decorative.** The page follows `prefers-color-scheme` when no
   theme is chosen; a chosen theme is remembered in `localStorage` and can be
   linked (`?theme=phosphor`). Motion is **always on** (DS-1.1 removed the
   reduced-motion gate by product decision), but it is small: one transition per
   state change, no parallax, nothing that loops except the cursor and the
   product window, which pauses under the pointer and off screen.

## Tokens: the ten colors

| Token | Used for | Example (`blueprint`) |
|---|---|---|
| `--bg` | page background | `#050912` |
| `--panel` | code frames, inputs, hover fills, key caps | `#0a1222` |
| `--hatch` | the hatched margin, the footer mark | `#0b1527` |
| `--line` | block separation, grids | `#131f36` |
| `--line-2` | frames, inputs, crosshairs, key-cap borders | `#22365b` |
| `--dim` | captions, hints, secondary text | `#6f86ab` |
| `--fg-2` | body text, prose | `#a3b5d3` |
| `--fg` | headings and emphasis | `#e8f0fd` |
| `--accent` | marks, the active state, primary buttons, keywords in code | `#6cb4ff` |
| `--ink` | text on an accent fill | `#050912` |

Code is highlighted by Shiki in its `css-variables` mode: the highlighter names the
kind of each token and `tokens.css` paints them with the active palette
(keywords and constants in the accent, strings and functions in the foreground,
comments dim), so a code block is monochrome in every theme with no overrides.

## Themes

| id | keys | accent | source of the palette |
|---|---|---|---|
| `blueprint` | dark | `#6cb4ff` | the default: blue ink on drafting paper |
| `graphite` | dark | `#ffffff` | black and white, a printed page |
| `phosphor` | dark | `#3dff85` | a green-screen terminal |
| `amber` | dark | `#ffb224` | an amber-screen terminal |
| `paper` | light | `#1550c8` | navy ink on a light sheet |

A theme is one block in `web/src/styles/tokens.css` (`html[data-theme="…"]`), one
entry in `THEMES` in `web/src/data/themes.ts` (for the chips and the scripts),
and nothing else — there is no theme-specific markup.

## Type

| Role | Face | Size | Notes |
|---|---|---|---|
| hero | Geist Mono 600 | `clamp(42px, 8.4vw, 108px)` | tracking `-0.065em`, line height `0.95` |
| section heading | Geist Mono 600 | `clamp(26px, 3.6vw, 40px)` | tracking `-0.04em` |
| interface | Geist Mono 400–500 | `11–14px` | labels are uppercase with `0.08–0.14em` tracking |
| lead | Geist 400 | `16–18px` | `65ch` at most, balanced wrapping |
| prose | Geist 400 | `16.5px` / `1.78` | `700px` column, `text-wrap: pretty` |
| code | Geist Mono 400 | `13px` / `1.7` | tab size 4 |

## Layout

- **`.frame`** — the page: `1240px` wide, centered, between two `1px` rails, over
  the hatched `.gutter` (a fixed SVG pattern drawn with a token).
- **`.sheet`** — a band of the frame. Its top rule carries the crosshairs; its
  `.sheet-in` pads it. Landing sections are sheets, numbered `[01]`…`[06]` with
  the `.sec-label`.
- **`.bp`** — the blueprint grid (`components/Blueprint.astro`), an inline SVG
  pattern in `--line` and `--line-2`, behind the hero, the closing band, the
  docs hub and the 404.
- **`.docs`** — the documentation shell, three columns: the document's pages on
  the left (`.aside`), the text in the middle, *on this page* on the right
  (`.toc`). Below `1140px` the right column goes away; below `820px` the left one
  becomes a drawer under a sticky *contents* bar.

## Components

- **`.head`** — the header: the `wr` mark with a block cursor, the version, the
  five destinations (the current one has an accent underline that slides in),
  the search trigger with its key caps (`⌘K` on a Mac), the theme dot and
  GitHub. A progress line in the accent sits on its bottom edge. On a narrow
  screen the destinations move into `.mnav`.
- **`.status`** — a status line fixed to the bottom, like a terminal multiplexer:
  a mode block (`WR`, `GUIDE`, `REF`, …), the path (`~/docs/guide/start`), then
  `/ search`, `? keys`, the reading position and the version.
- **`.pal`** — the dialog shape shared by the command palette (`data-palette`),
  the theme dialog (`data-theme-modal`) and the keyboard map (`data-help`). The
  selected row is inverted with the accent, the way the program highlights a
  line. The palette searches the *text* of every page (the same index as the
  search page) and also lists the pages, the guide's sections, the themes and a
  few actions; items with a destination are real links without JavaScript.
- **`.install`** — the install box: tabs (`script`, `go install`, `source`), one
  command, a copy button and a one-line note.
- **`.win`** — a terminal window; `#reel` is the product window, whose tabs are
  the recorded scenes and whose command line types the key that produced each
  frame.
- **`.vs`** — the noise-versus-signal demo: the clutter of a page is struck out
  and removed piece by piece the first time it is seen.
- **`.cells`, `.steps`, `.keys-grid`, `.tiles`, `.doors`** — `1px`-gap grids on a
  line-colored background, so the separation is the line of the grid itself.
- **`.prose`** — rendered Markdown: `§` before each `##` with a short accent bar
  on its rule, `–` and `01` list markers, key caps for `<kbd>`, tables in a
  scrolling wrapper, blockquotes as a ruled panel with an accent edge, headings
  that reveal a `#` link, and code frames with a language label and a copy
  button.
- **`.pager`** — previous and next page of a document, with the `[` and `]` keys.

## Motion

| Where | What | Notes |
|---|---|---|
| `.rise` | the hero copy entering, staggered | `0.7s`, `--d` steps of `70ms` |
| `.rv` | scroll reveals for the rest of the page | once, `12px` up, marked only when JavaScript runs |
| `[data-count]` | the four numbers counting up | `900ms`, ease-out |
| `.cur` | the terminal cursor block | `steps(1)`, `1.06s` |
| `#reel` | the product window: scenes crossfading with a progress line | one scene every `5.2s`; pauses on hover, focus and off screen |
| `.win-cmd` | the command typed under the window | `32ms` per character |
| `.nz` | the page clutter struck out and collapsed | `300ms` between pieces |
| `.head-nav a::after` | the active underline sliding in | `0.28s` |
| `.progress` | reading position on the header edge | transform only, no layout |
| `.pal-box` | dialogs entering | `0.18s` |
| view transition | page change | `90ms` out, `200ms` in |

Every transition animates `opacity`, `transform` or a color; nothing animates
layout except the collapse of the clutter in `.vs`.

## Keyboard

| Key | Does |
|---|---|
| `/` or `Ctrl`+`K` | search the docs / open the command palette |
| `[` `]` | previous / next page of the document |
| `t` | change the theme (arrows preview it live, `Enter` keeps it) |
| `g` `h` · `g` `d` | go home · go to the documentation |
| `?` | the keyboard map |
| `Esc` | close a dialog, leave a field |

On the search page `↑` `↓` choose a result and `Enter` opens it. None of the keys
fires while you type in a field.

## The recordings

`media/` holds the sources that produce the frames used above:

- `media/*.tape` — VHS tapes (ui, links, sections, shortcuts, search);
- `media/demo.toml` — the pinned `wr` config used for every recording;
- `media/scenes.json` — the same scenes as a PNG/SVG capture list, so the
  assets can be regenerated without VHS;
- `scripts/record.sh` — runs the tapes with VHS → `media/out/*.gif`;
- `scripts/capture.sh` — runs the scenes through the real program in a PTY and
  converts each frame to SVG → `web/public/assets/scenes/*.svg`.

New assets carry the `wr` version in `media/ASSETS.md` (generated by the
scripts), so a recording is always traceable to the commit that made it.
