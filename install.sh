#!/usr/bin/env bash
# linux-ip installer — single-script + curl distribution.
# Usage: curl -fsSL https://raw.githubusercontent.com/laneyweb/linux-ip/main/install.sh | bash
# Env: REPO=laneyweb/linux-ip  VERSION=v0.1.0 (or latest)  PREFIX=/usr/local/bin
set -euo pipefail

REPO="${REPO:-laneyweb/linux-ip}"
PREFIX="${PREFIX:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "x86_64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "unsupported arch: $(uname -m)" >&2; exit 1 ;;
  esac
}

tag="$VERSION"
if [ "$tag" = "latest" ]; then
  tag="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)"
  [ -n "$tag" ] || { echo "could not resolve latest release" >&2; exit 1; }
fi

A="$(arch)"
URL="https://github.com/${REPO}/releases/download/${tag}/linux-ip_${tag#v}_linux_${A}.tar.gz"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "→ downloading $URL"
curl -fsSL -o "$TMP/linux-ip.tar.gz" "$URL"
# checksum verify when .sha256 is published
if curl -fsSL -o "$TMP/sha256" "${URL}.sha256" 2>/dev/null; then
  (cd "$TMP" && sha256sum -c sha256 --status) && echo "→ checksum ok" || { echo "checksum FAILED" >&2; exit 1; }
fi
tar -xzf "$TMP/linux-ip.tar.gz" -C "$TMP"

BIN="$TMP/linux-ip"
[ -x "$BIN" ] || BIN="$(find "$TMP" -name linux-ip -type f | head -1)"
install -m 0755 "$BIN" "$PREFIX/linux-ip"
echo "✓ installed $("$PREFIX/linux-ip" --version) to $PREFIX/linux-ip"
echo "  try: linux-ip --help"
