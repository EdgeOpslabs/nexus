#!/usr/bin/env bash
set -euo pipefail

APPIMAGETOOL_X86="${1:-appimagetool}"
APPIMAGETOOL_ARM="${2:-appimagetool-aarch64}"
DIST_DIR="${DIST_DIR:-dist}"
HOST_ARCH="$(uname -m)"

build_appimage() {
  local arch="$1"
  local tool="$2"
  local tarball="${DIST_DIR}/nexus_linux_${arch}.tar.gz"
  local appdir="${DIST_DIR}/appimage/nexus_linux_${arch}.AppDir"
  local out="${DIST_DIR}/nexus_linux_${arch}.AppImage"

  if [[ ! -f "${tarball}" ]]; then
    echo "Missing tarball: ${tarball}"
    exit 1
  fi

  rm -rf "${appdir}"
  mkdir -p "${appdir}/usr/bin"
  mkdir -p "${DIST_DIR}/appimage"

  local tmp
  tmp="$(mktemp -d)"
  tar -xzf "${tarball}" -C "${tmp}"
  if [[ ! -f "${tmp}/nexus" ]]; then
    echo "Binary not found in ${tarball}"
    exit 1
  fi

  mv "${tmp}/nexus" "${appdir}/usr/bin/nexus"
  rm -rf "${tmp}"

  cat > "${appdir}/AppRun" <<'EOF'
#!/usr/bin/env bash
exec "$(dirname "$0")/usr/bin/nexus" "$@"
EOF
  chmod +x "${appdir}/AppRun"

  cat > "${appdir}/nexus.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=Nexus
Exec=nexus
Icon=nexus
Terminal=true
Categories=Development;
EOF

  cp -f "./packaging/appimage/nexus.svg" "${appdir}/nexus.svg"

  "${tool}" "${appdir}" "${out}"
}

if [[ "${HOST_ARCH}" == "x86_64" ]]; then
  build_appimage "amd64" "${APPIMAGETOOL_X86}"
  echo "Skipping arm64 AppImage build on x86_64 runner"
elif [[ "${HOST_ARCH}" == "aarch64" || "${HOST_ARCH}" == "arm64" ]]; then
  build_appimage "arm64" "${APPIMAGETOOL_ARM}"
  echo "Skipping amd64 AppImage build on arm64 runner"
else
  echo "Unsupported host architecture: ${HOST_ARCH}"
  exit 1
fi
