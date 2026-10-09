# Changelog

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
