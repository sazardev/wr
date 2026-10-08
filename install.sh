#!/bin/sh
# Install wr from the latest GitHub release.
#
#   curl -fsSL https://raw.githubusercontent.com/sazardev/wr/main/install.sh | sh
#
# Environment:
#   INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#   WR_VERSION   a specific release, e.g. v0.1.0 (default: the latest)
set -eu

REPO="sazardev/wr"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
die() { printf 'install: %s\n' "$*" >&2; exit 1; }

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
	linux|darwin) ;;
	*) die "unsupported system '$os' (wr has builds for Linux and macOS; on Windows use WSL)" ;;
esac
case "$(uname -m)" in
	x86_64|amd64) arch=amd64 ;;
	aarch64|arm64) arch=arm64 ;;
	*) die "unsupported CPU '$(uname -m)'" ;;
esac

asset="wr_${os}_${arch}.tar.gz"
if [ -n "${WR_VERSION:-}" ]; then
	base="https://github.com/$REPO/releases/download/$WR_VERSION"
else
	base="https://github.com/$REPO/releases/latest/download"
fi

if command -v curl >/dev/null 2>&1; then
	fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
	fetch() { wget -qO "$2" "$1"; }
else
	die "need curl or wget"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

say "downloading $asset ..."
fetch "$base/$asset" "$tmp/$asset" || die "could not download $base/$asset"
fetch "$base/checksums.txt" "$tmp/checksums.txt" || die "could not download the checksums"

want="$(grep " $asset\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
[ -n "$want" ] || die "no checksum listed for $asset"
if command -v sha256sum >/dev/null 2>&1; then
	got="$(sha256sum "$tmp/$asset" | cut -d' ' -f1)"
else
	got="$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)"
fi
[ "$want" = "$got" ] || die "checksum mismatch for $asset (expected $want, got $got)"
say "checksum ok"

mkdir -p "$tmp/x"
tar -xzf "$tmp/$asset" -C "$tmp/x"
[ -f "$tmp/x/wr" ] || die "the archive does not contain the wr binary"
mkdir -p "$INSTALL_DIR"
install -m 755 "$tmp/x/wr" "$INSTALL_DIR/wr"

say "installed: $INSTALL_DIR/wr ($("$INSTALL_DIR/wr" --version))"
case ":$PATH:" in
	*":$INSTALL_DIR:"*) ;;
	*) say "note: $INSTALL_DIR is not in your PATH; add it, e.g.  export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac
say "try:  wr https://example.com    (or just: wr)"
