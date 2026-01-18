#!/usr/bin/env bash
set -euo pipefail

TAP_REPO="${TAP_REPO:-EdgeOpslabs/homebrew-nexus}"
FORMULA_PATH="${FORMULA_PATH:-Formula/nexus-cli.rb}"
RELEASE_REPO="${RELEASE_REPO:-${GITHUB_REPOSITORY}}"
VERSION="${VERSION:-${GITHUB_REF_NAME}}"
TAP_BRANCH="${TAP_BRANCH:-main}"

if [[ -z "${HOMEBREW_TAP_TOKEN:-}" ]]; then
  echo "HOMEBREW_TAP_TOKEN is not set"
  exit 1
fi

if [[ -z "${VERSION}" ]]; then
  echo "VERSION is required"
  exit 1
fi

CHECKSUMS_URL="https://github.com/${RELEASE_REPO}/releases/download/${VERSION}/checksums.txt"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

curl -fsSL -H "Authorization: token ${GITHUB_TOKEN:-}" "${CHECKSUMS_URL}" -o "${TMP_DIR}/checksums.txt"

SHA_AMD64="$(grep "nexus_darwin_amd64.tar.gz" "${TMP_DIR}/checksums.txt" | awk '{print $1}')"
SHA_ARM64="$(grep "nexus_darwin_arm64.tar.gz" "${TMP_DIR}/checksums.txt" | awk '{print $1}')"

if [[ -z "${SHA_AMD64}" || -z "${SHA_ARM64}" ]]; then
  echo "Missing checksums for darwin amd64/arm64"
  exit 1
fi

git clone "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${TAP_REPO}.git" "${TMP_DIR}/tap"
cd "${TMP_DIR}/tap"
git checkout "${TAP_BRANCH}"

if [[ ! -f "${FORMULA_PATH}" ]]; then
  echo "Formula not found: ${FORMULA_PATH}"
  exit 1
fi

python3 - <<PY
import re
from pathlib import Path

path = Path("${FORMULA_PATH}")
text = path.read_text()

arm_url = f"https://github.com/${RELEASE_REPO}/releases/download/${VERSION}/nexus_darwin_arm64.tar.gz"
amd_url = f"https://github.com/${RELEASE_REPO}/releases/download/${VERSION}/nexus_darwin_amd64.tar.gz"
arm_sha = "${SHA_ARM64}"
amd_sha = "${SHA_AMD64}"
version = "${VERSION#v}"

text = re.sub(
    r"(if Hardware::CPU\.arm\?\s*\n\s*url \")[^\"]+(\")\s*\n\s*sha256 \"[^\"]+\"",
    rf"\\1{arm_url}\\2\n    sha256 \"{arm_sha}\"",
    text,
)
text = re.sub(
    r"(else\s*\n\s*url \")[^\"]+(\")\s*\n\s*sha256 \"[^\"]+\"",
    rf"\\1{amd_url}\\2\n    sha256 \"{amd_sha}\"",
    text,
)
text = re.sub(r"(version \")[^\"]+(\")", rf"\\1{version}\\2", text)

path.write_text(text)
PY
rm -f "${FORMULA_PATH}.bak"

git add "${FORMULA_PATH}"
git -c user.name="EdgeOpslabs CI" -c user.email="ci@edgeopslabs.com" commit -m "Update nexus-cli to ${VERSION}"
git push origin "${TAP_BRANCH}"
