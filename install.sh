#!/bin/sh
# install.sh — fetch the latest kete release binary and drop it on PATH.
#
# Usage:
#   # private repo (current — needs gh CLI authenticated):
#   gh release download v0.1.0 --repo dreamware-nz/kete \
#     --pattern install.sh --output - | sh
#
#   # public repo (post-public-release):
#   curl -fsSL https://raw.githubusercontent.com/dreamware-nz/kete/main/install.sh | sh
#
# Honours:
#   PREFIX   — install dir (default $HOME/.local)
#   VERSION  — release tag to install (default: latest)
#
# Refuses to run with sudo by default — kete is per-user.

set -eu

PREFIX="${PREFIX:-$HOME/.local}"
VERSION="${1:-${VERSION:-latest}}"
REPO="dreamware-nz/kete"

# --- detect platform ---

uname_s=$(uname -s)
uname_m=$(uname -m)

case "$uname_s" in
  Darwin) os=darwin ;;
  Linux)  os=linux ;;
  *)
    echo "kete: unsupported OS: $uname_s" >&2
    exit 1
    ;;
esac

case "$uname_m" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *)
    echo "kete: unsupported arch: $uname_m" >&2
    exit 1
    ;;
esac

# --- resolve version ---

if [ "$VERSION" = "latest" ]; then
  if command -v gh >/dev/null 2>&1; then
    VERSION=$(gh release view --repo "$REPO" --json tagName -q .tagName 2>/dev/null || true)
  fi
  if [ -z "$VERSION" ] || [ "$VERSION" = "latest" ]; then
    # fall back to the GitHub API
    VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
      | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
      | head -1)
  fi
  if [ -z "$VERSION" ]; then
    echo "kete: could not resolve latest release tag" >&2
    exit 1
  fi
fi

# --- fetch ---

binary="kete-${os}-${arch}"
url="https://github.com/$REPO/releases/download/${VERSION}/${binary}"
shaurl="https://github.com/$REPO/releases/download/${VERSION}/SHA256SUMS"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

echo "kete: downloading $VERSION ($binary)"
# Try anonymous curl first (works for public repos); fall back to
# `gh release download` (works for private repos via the user's
# already-authenticated gh CLI).
if ! curl -fsSL --output "$tmpdir/$binary" "$url" 2>/dev/null; then
  if command -v gh >/dev/null 2>&1; then
    echo "kete: anonymous download failed, retrying via gh CLI"
    if ! gh release download "$VERSION" \
      --repo "$REPO" \
      --pattern "$binary" \
      --dir "$tmpdir"; then
      echo "kete: gh release download failed" >&2
      exit 1
    fi
  else
    echo "kete: download failed and gh CLI not installed" >&2
    echo "  install gh (https://cli.github.com) or download manually:" >&2
    echo "    $url" >&2
    exit 1
  fi
fi

# --- verify (best-effort; SHA256SUMS may not exist on older releases) ---

if curl -fsSL --output "$tmpdir/SHA256SUMS" "$shaurl" 2>/dev/null \
   || gh release download "$VERSION" --repo "$REPO" --pattern SHA256SUMS --dir "$tmpdir" 2>/dev/null; then
  expected=$(grep " $binary\$" "$tmpdir/SHA256SUMS" | awk '{print $1}')
  if [ -n "$expected" ]; then
    if command -v shasum >/dev/null 2>&1; then
      actual=$(shasum -a 256 "$tmpdir/$binary" | awk '{print $1}')
    else
      actual=$(sha256sum "$tmpdir/$binary" | awk '{print $1}')
    fi
    if [ "$expected" != "$actual" ]; then
      echo "kete: checksum mismatch" >&2
      echo "  expected: $expected" >&2
      echo "  got:      $actual" >&2
      exit 1
    fi
    echo "kete: checksum ok"
  fi
fi

# --- install ---

mkdir -p "$PREFIX/bin"
install -m 0755 "$tmpdir/$binary" "$PREFIX/bin/kete"

echo "kete: installed $PREFIX/bin/kete"
"$PREFIX/bin/kete" --version || true

# --- PATH hint ---

case ":$PATH:" in
  *":$PREFIX/bin:"*) ;;
  *)
    echo
    echo "kete: $PREFIX/bin is not on your PATH"
    echo "  add this to your shell rc:"
    echo "    export PATH=\"$PREFIX/bin:\$PATH\""
    ;;
esac
