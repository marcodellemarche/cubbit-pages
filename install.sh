#!/usr/bin/env bash
set -euo pipefail

REPO="marcodellemarche/cubbit-pages"
BINARY="cubbit-pages"
VERSION="${1:-latest}"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux*)  OS="linux" ;;
  darwin*) OS="darwin" ;;
  *)       echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

FILENAME="${BINARY}-${OS}-${ARCH}"

if [ "$VERSION" = "latest" ]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${FILENAME}"
  CHECKSUM_URL="https://github.com/${REPO}/releases/latest/download/${FILENAME}.sha256"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"
  CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}.sha256"
fi

echo "Downloading ${BINARY} ${VERSION} for ${OS}/${ARCH}..."

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Download binary and checksum
curl -fsSL -o "${TMPDIR}/${FILENAME}" "${DOWNLOAD_URL}"
curl -fsSL -o "${TMPDIR}/${FILENAME}.sha256" "${CHECKSUM_URL}" 2>/dev/null || true

# Verify checksum if available
if [ -f "${TMPDIR}/${FILENAME}.sha256" ] && [ -s "${TMPDIR}/${FILENAME}.sha256" ]; then
  echo "Verifying checksum..."
  cd "${TMPDIR}"
  if command -v sha256sum &>/dev/null; then
    sha256sum -c "${FILENAME}.sha256"
  elif command -v shasum &>/dev/null; then
    shasum -a 256 -c "${FILENAME}.sha256"
  else
    echo "Warning: no sha256 tool found, skipping checksum verification"
  fi
  cd - >/dev/null
fi

chmod +x "${TMPDIR}/${FILENAME}"

# Install
INSTALL_DIR="/usr/local/bin"
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/${FILENAME}" "${INSTALL_DIR}/${BINARY}"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  if command -v sudo &>/dev/null; then
    sudo mv "${TMPDIR}/${FILENAME}" "${INSTALL_DIR}/${BINARY}"
  else
    INSTALL_DIR="${HOME}/bin"
    mkdir -p "$INSTALL_DIR"
    mv "${TMPDIR}/${FILENAME}" "${INSTALL_DIR}/${BINARY}"
    echo "Installed to ${INSTALL_DIR}/${BINARY}"
    echo "Make sure ${INSTALL_DIR} is in your PATH"
  fi
fi

echo "cubbit-pages installed successfully!"
"${INSTALL_DIR}/${BINARY}" version
