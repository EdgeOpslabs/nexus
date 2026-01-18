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

sed -i.bak "s|url \".*nexus_darwin_arm64.tar.gz\"|url \"https://github.com/${RELEASE_REPO}/releases/download/${VERSION}/nexus_darwin_arm64.tar.gz\"|g" "${FORMULA_PATH}"
sed -i.bak "s|sha256 \".*\"|sha256 \"${SHA_ARM64}\"|g" "${FORMULA_PATH}"
sed -i.bak "0,/url \".*nexus_darwin_amd64.tar.gz\"/s|url \".*nexus_darwin_amd64.tar.gz\"|url \"https://github.com/${RELEASE_REPO}/releases/download/${VERSION}/nexus_darwin_amd64.tar.gz\"|" "${FORMULA_PATH}"
sed -i.bak "0,/sha256 \".*\"/s|sha256 \".*\"|sha256 \"${SHA_AMD64}\"|" "${FORMULA_PATH}"
sed -i.bak "s|version \".*\"|version \"${VERSION#v}\"|" "${FORMULA_PATH}"
rm -f "${FORMULA_PATH}.bak"

git add "${FORMULA_PATH}"
git -c user.name="EdgeOpslabs CI" -c user.email="ci@edgeopslabs.com" commit -m "Update nexus-cli to ${VERSION}"
git push origin "${TAP_BRANCH}"
