# Changelog

## [0.9.0](https://github.com/sazardev/wr/compare/v0.8.0...v0.9.0) (2026-10-12)


### Features

* move between search matches while typing with up/down, tab or ctrl+n/p ([3f866f3](https://github.com/sazardev/wr/commit/3f866f38320abeec4aee07e2629941f38f6bd829))

## [0.8.0](https://github.com/sazardev/wr/compare/v0.7.0...v0.8.0) (2026-10-12)


### Features

* find options with / in the menu, Settings, shortcuts and the index; drop the footer version ([eff7897](https://github.com/sazardev/wr/commit/eff78978b402c4be9d99408c2fc02e890874c7a0))

## [0.7.0](https://github.com/sazardev/wr/compare/v0.6.0...v0.7.0) (2026-10-12)


### Features

* check for new versions, update from inside wr and show what's new ([9b41589](https://github.com/sazardev/wr/commit/9b41589e430bf4ee87a2b2b0135c21f78a178b9c))

## [0.6.0](https://github.com/sazardev/wr/compare/v0.5.1...v0.6.0) (2026-10-12)


### Features

* **mcp:** agent harness — MCP server, headless sessions and eval ([670841b](https://github.com/sazardev/wr/commit/670841b54f0348a783893645f08ae2652c72e066))
* read script-drawn pages, guess code languages and polish extraction ([9420c56](https://github.com/sazardev/wr/commit/9420c56ed71bea90c6f63402739bf53426a34c29))
* **web:** blueprint redesign of the docs site (DS-2.0) ([29db8e3](https://github.com/sazardev/wr/commit/29db8e3b51e5b70db359cba9e3792ac811ccb990))


### Performance Improvements

* faster extraction, search and render in three single-pass rewrites ([98e23e9](https://github.com/sazardev/wr/commit/98e23e9f6de2e5c2a297fd2d2684de83184e952e))

## [0.5.1](https://github.com/sazardev/wr/compare/v0.5.0...v0.5.1) (2026-10-12)


### Bug Fixes

* **deps:** bump github.com/yuin/goldmark ([#3](https://github.com/sazardev/wr/issues/3)) ([1cc95bb](https://github.com/sazardev/wr/commit/1cc95bbae1747ed870be305e2542b5677a7f97d5))

## [0.5.0](https://github.com/sazardev/wr/compare/v0.4.0...v0.5.0) (2026-10-11)


### Features

* **web:** command palette for the docs site ([#20](https://github.com/sazardev/wr/issues/20)) ([d9bb4c0](https://github.com/sazardev/wr/commit/d9bb4c0919fc06ad787183bd02825963eb53ed3c))

## [0.4.0](https://github.com/sazardev/wr/compare/v0.3.0...v0.4.0) (2026-10-10)


### Features

* give the accent a partner color and name the status colors ([6e5397d](https://github.com/sazardev/wr/commit/6e5397d3f46b05b41c841c3aa0fc96031644e91d))

## [0.3.0](https://github.com/sazardev/wr/compare/v0.2.1...v0.3.0) (2026-10-09)


### Features

* give --help the reader's look, driven by one flag table ([f4df27d](https://github.com/sazardev/wr/commit/f4df27d8bf085cb2f2a379cd9539a74284a37579))

## [0.2.1](https://github.com/sazardev/wr/compare/v0.2.0...v0.2.1) (2026-10-09)


### Bug Fixes

* stop losing pages and rewriting code during extraction ([6327651](https://github.com/sazardev/wr/commit/6327651d5a8603d7ba0b127161d898f550d5e452))

## [0.2.0](https://github.com/sazardev/wr/compare/v0.1.0...v0.2.0) (2026-10-09)


### Features

* add the installer, guide and automated releases ([edc2be8](https://github.com/sazardev/wr/commit/edc2be802abea3b8f6cee0a74181b24569902321))

## v0.1.0

First tagged release.

- Reader: formatted Markdown, per-language code highlighting, search, section
  index, select-and-copy (code copies without its frame), a one-row footer.
- Browsing: clickable links (mouse, Tab/Enter, `f` hint labels), back/forward
  that restore the scroll position, an address bar that also searches the web,
  persistent history and bookmarks, a start page, Markdown/text/JSON support.
- Customization from inside the program: a Settings panel (~30 options, applied
  live and saved) and rebindable key bindings.
- Motion: spring-physics scrolling, page reveal, shutter panels, loading wave,
  quick notices; each animation has its own switch and nothing runs at rest.
- Disk cache with background refresh, written atomically.
