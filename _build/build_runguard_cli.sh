#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   ./_build/build_runguard_cli.sh
#
# Reads _build/version.txt as MAJOR.MINOR.PATCH,
# increments PATCH, builds binaries, writes new version to _build/version_new.txt

VERSIONFILE="_build/version.txt"
NEWVERSIONFILE="_build/version_new.txt"

# Config is overridable via env vars so this script works unchanged from a
# developer's machine or from CI (see .github/workflows/deploy.yml).
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="${REMOTE_HOST:-manage}"
SSH_KEY="${SSH_KEY:-$HOME/Downloads/ssh_keys/enyim}"


if [[ ! -f "${VERSIONFILE}" ]]; then
  echo "version file not found: ${VERSIONFILE}" >&2
  exit 1
fi

IFS='.' read -r MAJOR MINOR PATCH <<<"$(cat "${VERSIONFILE}")"
echo "MAJOR: ${MAJOR}"
echo "MINOR: ${MINOR}"
PATCH=$((PATCH + 1))
echo "PATCH: ${PATCH}"
VERSION="${MAJOR}.${MINOR}.${PATCH}"
echo "New version: ${VERSION}"

echo "${VERSION}" > "${NEWVERSIONFILE}"

REMOTE_DIR="/srv/runguard-downloads/${VERSION}"

# Figure out repo root and backend module root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
BACKEND_DIR="${PROJECT_ROOT}/backend"

# Where to drop binaries
BUILD_ROOT="${PROJECT_ROOT}/tmp_backend_cli"
BUILD_DIR="${BUILD_ROOT}/${VERSION}"

echo "==> Building RunGuard CLI ${VERSION} into ${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

cleanup() {
  if [[ -n "${BUILD_ROOT:-}" && -d "${BUILD_ROOT}" ]]; then
    echo "Cleaning up temp dir: ${BUILD_ROOT}"
    rm -rf -- "${BUILD_ROOT}"
  fi
}
trap cleanup EXIT

# Matrix of targets
targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
)

echo "Ensure remote dir exists..."
ssh -i "$SSH_KEY" $REMOTE_USER@$REMOTE_HOST "mkdir -p $REMOTE_DIR"

for t in "${targets[@]}"; do
  read -r GOOS GOARCH <<<"${t}"

  BIN_NAME="runguard-${GOOS}-${GOARCH}-${VERSION}"
  OUT_PATH="${BUILD_DIR}/${BIN_NAME}"

  echo "  -> ${GOOS}/${GOARCH} -> ${OUT_PATH}"

  # Build from backend module root (where go.mod is)
  GOOS="${GOOS}" GOARCH="${GOARCH}" CGO_ENABLED=0 \
    go -C "${BACKEND_DIR}" build -v -ldflags="-s -w" \
      -o "${OUT_PATH}" ./cmd/runguard

  chmod +x "${OUT_PATH}"
  echo "Uploading new runguard binary..."
  scp -i "$SSH_KEY" "${OUT_PATH}" $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/${BIN_NAME}

  # SHA256 (not MD5): actually usable for tamper-detection, not just
  # corrupted-download detection. install.sh and self-update both verify
  # against this and now abort on mismatch instead of just warning.
  echo "Create .sha256 file..."
  (
    cd "${BUILD_DIR}"
    sha256sum "${BIN_NAME}" > "${BIN_NAME}.sha256"
  )
  echo "Uploading .sha256 file..."
  scp -i "$SSH_KEY" "${OUT_PATH}.sha256" $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/${BIN_NAME}.sha256


done

echo "==> Done. Binaries are in: ${BUILD_DIR}"
echo "    Version written to: ${NEWVERSIONFILE}"
mv ${NEWVERSIONFILE} ${VERSIONFILE}
