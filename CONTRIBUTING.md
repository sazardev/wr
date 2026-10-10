# Contributing to wr

Thanks for wanting to make `wr` better. This project is open source under the
[MIT license](LICENSE): use it, study it, change it, fork it, ship it, sell it —
the only thing I ask is that you keep the copyright notice.

## Ways to help

- Report bugs and suggest features in [issues](https://github.com/sazardev/wr/issues).
- Send fixes and features as pull requests. Small, focused PRs are easiest to
  review and merge.
- Improve the docs: the README, the [guide](docs/GUIDE.md) and the changelog
  (the last one is generated, see [Releases](#releases)). The published site
  (`web/`) is built from them, so editing `docs/GUIDE.md` edits the website.

## Development

```sh
git clone https://github.com/sazardev/wr
cd wr
sh scripts/setup-hooks.sh   # enables the git hooks (once per clone)
go test -race ./...
go build .
```

The hooks are versioned in `.githooks/` and run the same checks as CI:

- **pre-commit** formats the staged Go files with `gofmt`, then runs `go vet`
  and `go test`.
- **pre-push** runs `go test -race ./...` and `go build ./...`.

You can bypass them with `--no-verify` if you must, but CI will run them anyway.

CI (`.github/workflows/ci.yml`) checks `gofmt`, `go mod tidy -diff`, `go vet`,
`go test -race` and `go build`. GitHub Actions are pinned to commit SHAs (the
repository enforces it); Dependabot keeps them updated.

## The documentation site

The site at <https://sazardev.github.io/wr/> is an [Astro](https://astro.build/)
app in [`web/`](web/), deployed to GitHub Pages by
`.github/workflows/docs.yml`. It is not hand-written HTML: a custom content
loader reads `docs/GUIDE.md`, `docs/DESIGN.md` and `CHANGELOG.md` from this
repository and splits them into **one page per section** (`/docs/guide/start/`,
`/docs/guide/following-links/`, …). The reference pages — every shortcut, every
setting — are generated at build time from `keys.go` and the README, and the
search page keeps its state in the query string (`/docs/search/?q=links`).

The documentation's version is the repository's version: the deploy reads
`.release-please-manifest.json`, so a release bumps the docs automatically.

```sh
cd web
npm ci          # once (package-lock.json is committed)
npm run dev     # the site at /wr/ with a live reload
npm run verify  # astro check + the unit tests + a production build
```

Everything that is not a component lives in `web/src/lib/` as pure, tested
modules: the section splitter, the table and Go parsers, the search scorer.
Run them with `npm test`.

## Commit and PR titles

Commits (and PR titles, which become the squash commit) follow
[Conventional Commits](https://www.conventionalcommits.org/):

| Type | Use for | In the changelog |
|---|---|---|
| `feat` | a new feature | yes (minor bump) |
| `fix` | a bug fix | yes (patch bump) |
| `perf` | a performance improvement | yes |
| `deps` | dependency updates | yes |
| `refactor` `docs` `test` `ci` `build` `chore` | everything else | hidden |

`feat:`, `fix(render):`, `feat!:`. A breaking change (`!` or a
`BREAKING CHANGE:` footer) bumps the major version.

The CI checks the PR title, so a bad title fails the build before review.

## Review and merge

- `@sazardev` is the reviewer: every PR needs their approval and a green CI run
  before it can be merged into `main`.
- `main` is protected: no force pushes, no direct commits, linear history only
  (squash or rebase).
- Please keep the branch up to date with `main`; stale approvals are dismissed
  when new commits arrive.

## Releases

Releases are automated with [release-please](https://github.com/googleapis/release-please)
and need no manual version bump:

1. A conventional commit lands on `main` (usually by merging a PR).
2. release-please opens or updates a **Release PR** with the next version and
   the changelog generated from those commits.
3. When the maintainer merges that PR, a `vX.Y.Z` tag and a GitHub Release are
   created, and the release workflow builds the Linux/macOS archives with
   `scripts/release.sh`, attaches them plus `checksums.txt`, and verifies the
   tagged commit with `go test` first.

Nothing is published without the Release PR being reviewed and merged, so the
changelog always matches what actually shipped.

Because the Release PR is opened by a bot, GitHub holds its workflow runs for
approval, so the maintainer merges it with the admin bypass after reviewing the
diff; the release job runs `go test` on the tagged commit before building, so
nothing ships untested.
