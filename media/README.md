# media — the recorded UI

The frames the [docs site](../docs/) shows, and the sources that produce them.

| File | What it is |
|---|---|
| `demo.html` | the sample article the recordings open (self-contained: no network, so a frame never depends on a live site) |
| `demo.toml` | the pinned `wr` config for every recording: `100` columns, no animation, standard footer. Identical settings across versions of the terminal. |
| `scenes.json` | the same scenes as data, for the no-VHS capture path |
| `ui.tape`, `links.tape`, `sections.tape`, `shortcuts.tape`, `search.tape` | VHS tapes: reading, link labels, section index, menu and search |
| `ASSETS.md` | manifest with the `wr` version and the checksum of every generated frame |

## Two ways to regenerate everything

**1. Without VHS** (default: a real PTY, one scene at a time, each screen
converted to a flat SVG that the site embeds):

```sh
sh scripts/capture.sh          # -> docs/assets/scenes/*.svg + media/ASSETS.md
```

**2. With VHS** (real GIFs, good for a README or a release):

```sh
go install github.com/charmbracelet/vhs@latest   # also needs ffmpeg
sh scripts/record.sh                             # -> media/out/*.gif
```

Both paths use `media/demo.toml` and `media/demo.html`, so a GIF and an SVG of
the same scene show exactly the same screen.

## Scene list

| Scene | Keys | What it demonstrates |
|---|---|---|
| `ui` | `o` (address) `j j` `t` | reading an article: plain text, braille rules, footnote links, code frames |
| `links` | `f a 1` | every link labelled with a short hint, following one by typing it |
| `sections` | `t ]` | the heading index and section jumps |
| `shortcuts` | `m 7` | the menu: every bound key in one panel |
| `search` | `/ braille` `n n` | search as you type, highlighted matches, `n`/`N` between them |

Each tape pins its own shell state: it creates a temporary `HOME` with the demo
config before opening `wr`, so recordings never read the developer's config,
history, bookmarks or cache.

## Versioning

`media/ASSETS.md` is generated on every run and records the `wr` version
(`wr --version`), the commit the images were generated from and a checksum per
file. A frame is therefore always traceable to the build that produced it, and a
regenerated asset with a different checksum means a visible change.

## Adding a scene

1. Add it to `media/scenes.json` (name, keys, wait) — the SVG path works at once.
2. Add a `media/<name>.tape` if GIF output is wanted.
3. Reference the frame in `docs/index.html` and `docs/guide.html` (the
   `.demos` grid), and describe it above.
