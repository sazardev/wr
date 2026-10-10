# wr design system

The design language of the documentation site (the Astro app in `web/`) and of the
recordings in `media/`: a terminal look, flat, no decoration that a terminal
cannot draw itself. Version: **DS-1.2** (2026-10-09).

- [Rules](#rules)
- [Tokens: the eight colors](#tokens-the-eight-colors)
- [Themes](#themes)
- [Glyph inventory](#glyph-inventory)
- [Components](#components)
- [Motion](#motion)
- [The recordings](#the-recordings)

## Rules

1. **Flat.** No `border-radius`, no `box-shadow`, no `text-shadow`, no gradient
   (`linear-gradient` appears in no file), no `backdrop-filter`, no translucent
   panels: `rgba()` over backgrounds is limited to the braille field, where
   opacity over one accent color is the encoding of height.
2. **Terminals draw lines and glyphs.** Separation between blocks is a `1px`
   border or a braille rule (`⣀⣀⣀…`); a "window" is a box-drawing frame; a
   label is a caption row above the content, like the reader's code frames.
3. **Monospace everywhere.** One font stack, no second typeface. Hierarchy comes
   from color and letter-spacing, not from weight tricks.
4. **Eight colors per theme** (bg, surface, fg, dim, accent, accent2, line,
   line-strong). Nothing else is colored, and every theme keeps the same names.
5. **The program is the reference.** The footer hints, the braille progress bar,
   the section rules and the code frames are the same shapes `wr` draws, so the
   docs read like a page opened in the reader.
6. **Adaptive, not decorative.** The page follows `prefers-color-scheme` when no
   theme is chosen and a chosen theme is remembered in `localStorage`. Motion is
   **always on** (DS-1.1 removed the reduced-motion gate by product decision):
   the field, the reel, the marquee, the cursor and the progress bar animate by
   default.

## Tokens: the eight colors

| Token | Used for | Example (`wr` theme) |
|---|---|---|
| `--bg` | page background | `#07090d` |
| `--surface` | code frames, table headers, inputs | `#0d1117` |
| `--fg` | body text | `#c9d3e3` |
| `--dim` | captions, hints, secondary text | `#5f7089` |
| `--accent` | links, marks, keyboard glyphs, rules | `#4da3ff` |
| `--accent2` | secondary highlight, links on hover | `#7dd3fc` |
| `--line` | block separation | `#1b2534` |
| `--line-strong` | frames, inputs, active borders | `#24344a` |

## Themes

| id | keys | accent | accent2 | source of the palette |
|---|---|---|---|---|
| `wr` | dark | `#4da3ff` | `#7dd3fc` | the reader's own defaults |
| `gruvbox` | dark | `#fabd2f` | `#83a598` | gruvbox |
| `nord` | dark | `#88c0d0` | `#81a1c1` | nord |
| `catppuccin` | dark | `#cba6f7` | `#89b4fa` | catppuccin-mocha |
| `solarized` | dark | `#268bd2` | `#2aa198` | solarized dark |
| `matrix` | dark | `#2ee66f` | `#7bf7a4` | a phosphor terminal |
| `paper` | light | `#0b5cad` | `#0f766e` | light terminal on paper |

A theme is one block in `web/src/styles/site.css` (`html[data-theme="…"]`), one
entry in `THEMES` in `web/src/data/themes.ts` (for the chips and the scripts),
and nothing else — there is no theme-specific markup.

## Glyph inventory

| Glyph | Name | Use |
|---|---|---|
| `⣿` `⡇` `⣷` `⣄` `⣶` | braille icons | brand, section markers, key hints |
| `⣀ ⣄ ⣤ ⣦ ⣶ ⣷ ⣿` | fill ladder | section rules, the footer progress bar |
| `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` | spinner | "loading" states (hero prompt) |
| `⠁⠃⠇⡇⣇⣿` | single column | small icons |
| `╭─ ╰─ │ ┆ 🬖` | box drawing | code frames (drawn by the reader) |
| `▸ ▊ ⬡ ✓` | ASCII-adjacent | markers and cursor |

## Components

- **`.term`** — a window: `.term-bar` (dots, title, size on the right) over a
  `<pre>` body. Used for the install command and the screen capture.
- **`.rule-b`** — a `⣀` fill to the width of its section; JS sizes it
  (`web/src/scripts/rules.ts` → `fillRules`).
- **`.cards`** — a 1px-gap grid, so separation is the line of the grid itself.
- **`.demos` / `.demo`** — recorded frames with a caption row (`b` name,
  `.k` key).
- **`.swatches`** — one button per theme with three color chips; it calls
  `setTheme()`.
- **`.site-foot`** — the app's footer: hints on the left with the braille key,
  braille progress bar and percentage on the right.
- **`.reel`** — the hero: a `.term` frame whose `.reel-stage` crossfades the
  captured scenes while `.reel-cmd` types the command that produced each one
  (`data-name`, `data-cmd` in the HTML; `braille.js` only drives it).
- **`.strip`** — an infinite marquee of terminals and transports, flat and dim.
- **`.benefits`** — the sales row: four one-line benefits, 1px-gap grid.
- **`.rise`** — the entrance for the hero copy (`.d1`…`.d4` stagger).

## Motion

| Where | What | Notes |
|---|---|---|
| `#field` | braille height field, sink waves + click ripple | one accent color, opacity encodes height |
| `.cursor` | blinking braille cell with a 3-step trail | fine pointers only |
| `[data-spin]` | braille spinner in the hero prompt | 110 ms step |
| `.blink` | terminal cursor block | `steps(1)`, 1.06 s |
| `#progress` | scroll progress, cell by cell | follows the reader's bar |
| `.reel-frame` | the hero reel: captured scenes crossfading | one frame every 4.6 s |
| `.reel-cmd` | the command typed under the reel | 34 ms per character |
| `.strip-track` | the terminals marquee | 38 s loop, `translateX(-50%)` |
| `.rise` | hero copy entering, staggered | 0.7 s, 4 steps |
| `.brand .b` | the brand glyph breathing | 2.8 s |

Motion is always on; the field pauses when the tab is hidden and no frames are
drawn while the document is not visible.

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
