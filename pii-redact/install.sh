#!/bin/sh
set -e

REPO="enekos/mairu"
BINARY="pii-redact"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux) OS="linux" ;;
  darwin) OS="darwin" ;;
  msys*|cygwin*|mingw*|nt|win*) OS="windows" ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# If a version is not provided, use "latest"
VERSION="${VERSION:-latest}"

if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${BINARY}-${OS}-${ARCH}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}-${OS}-${ARCH}"
fi

# Windows gets .exe
if [ "$OS" = "windows" ]; then
  URL="${URL}.exe"
  BINARY="${BINARY}.exe"
fi

echo "Downloading ${BINARY} ${VERSION} for ${OS}/${ARCH}..."
TMPDIR="${TMPDIR:-/tmp}"
TMPFILE="${TMPDIR}/${BINARY}.download"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$TMPFILE" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "$TMPFILE" "$URL"
else
  echo "curl or wget is required"
  exit 1
fi

# Verify download succeeded
if [ ! -s "$TMPFILE" ]; then
  echo "Download failed or file is empty: $URL"
  rm -f "$TMPFILE"
  exit 1
fi

chmod +x "$TMPFILE"

# Install
echo "Installing to ${INSTALL_DIR}/${BINARY}..."
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
else
  if command -v sudo >/dev/null 2>&1; then
    # Check if we can sudo without a password (non-interactive / piped install)
    if sudo -n mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}" 2>/dev/null; then
      :
    else
      echo ""
      echo "Cannot write to ${INSTALL_DIR} without a password."
      echo "Either run with a writable INSTALL_DIR:"
      echo "  curl -sSL ... | INSTALL_DIR=\$HOME/.local/bin sh"
      echo "Or run the script directly with sudo:"
      echo "  sudo sh -c '\$(curl -sSL ...)'"
      rm -f "$TMPFILE"
      exit 1
    fi
  else
    echo "Cannot write to ${INSTALL_DIR}. Set INSTALL_DIR or run with sudo."
    rm -f "$TMPFILE"
    exit 1
  fi
fi

echo "Installed: $(${INSTALL_DIR}/${BINARY} --version 2>/dev/null || echo 'Run with --help to get started')"
