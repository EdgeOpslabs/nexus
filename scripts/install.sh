#!/usr/bin/env bash
set -euo pipefail

REPO="edgeopslabs/nexus-core"
VERSION="${VERSION:-v0.0.1}"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${ARCH}" in
  x86_64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
esac

if [[ "${OS}" == "darwin" ]]; then
  ASSET="nexus_darwin_${ARCH}.tar.gz"
elif [[ "${OS}" == "linux" ]]; then
  ASSET="nexus_linux_${ARCH}.tar.gz"
else
  echo "Unsupported OS: ${OS}"
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

echo "Downloading ${URL}"
curl -fsSL "${URL}" -o "${TMP_DIR}/${ASSET}"

tar -xzf "${TMP_DIR}/${ASSET}" -C "${TMP_DIR}"
chmod +x "${TMP_DIR}/nexus"

if [[ "${EUID}" -ne 0 ]]; then
  sudo mv "${TMP_DIR}/nexus" /usr/local/bin/nexus
else
  mv "${TMP_DIR}/nexus" /usr/local/bin/nexus
fi

echo "Installed nexus to /usr/local/bin/nexus"
