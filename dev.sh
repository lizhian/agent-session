#!/usr/bin/env bash
set -euo pipefail

fail() {
  printf 'Error: %s\n' "$1" >&2
  exit 1
}

info() {
  printf '%s\n' "$1"
}

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.agent-session/bin}"
BINARY_PATH="$INSTALL_DIR/agent-session"

command -v go >/dev/null 2>&1 || fail "go was not found in PATH"

mkdir -p "$INSTALL_DIR"

info "Building agent-session from $ROOT_DIR"
go build -ldflags='-s -w' -o "$BINARY_PATH" "$ROOT_DIR/cmd/agent-session/"

ln -sf "$BINARY_PATH" "$INSTALL_DIR/c"
ln -sf "$BINARY_PATH" "$INSTALL_DIR/cx"
ln -sf "$BINARY_PATH" "$INSTALL_DIR/oc"
ln -sf "$BINARY_PATH" "$INSTALL_DIR/p"

info "Installed local build to $BINARY_PATH"
info "Verify with:"
info "  $BINARY_PATH --help"
info "  $INSTALL_DIR/c --help"
info "  $INSTALL_DIR/cx --help"
