#!/bin/sh
# Build the release archives for a tag.
#
#   scripts/release.sh v0.1.0 [source-dir [docs-dir]]
#
# Builds static, trimmed binaries (CGO off) for Linux and macOS on amd64 and
# arm64 into ./dist, plus checksums.txt. Pass a checkout of the tag as the
# source dir to build exactly what was tagged; the README, changelog and guide
# packed next to the binary come from docs-dir (default: the source dir).
set -eu

VERSION="${1:?usage: scripts/release.sh vX.Y.Z [source-dir]}"
SRC="${2:-.}"
DOCS="${3:-$SRC}"
OUT="$(pwd)/dist"
rm -rf "$OUT" && mkdir -p "$OUT"

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
	os="${target%/*}"
	arch="${target#*/}"
	stage="$(mktemp -d)"
	(cd "$SRC" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
		go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$stage/wr" .)
	cp "$DOCS/LICENSE" "$DOCS/README.md" "$DOCS/CHANGELOG.md" "$stage/"
	mkdir -p "$stage/docs" && cp "$DOCS/docs/GUIDE.md" "$stage/docs/"
	tar -C "$stage" -czf "$OUT/wr_${os}_${arch}.tar.gz" wr LICENSE README.md CHANGELOG.md docs
	rm -rf "$stage"
	echo "built wr_${os}_${arch}.tar.gz"
done

(cd "$OUT" && if command -v sha256sum >/dev/null 2>&1; then sha256sum ./*.tar.gz; else shasum -a 256 ./*.tar.gz; fi |
	sed 's#\./##' > checksums.txt)
cat "$OUT/checksums.txt"
