#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   ./_build/build_runguard_cli.sh
#
# Reads _build/version.txt as MAJOR.MINOR.PATCH,
# increments PATCH, builds binaries, writes new version to _build/version_new.txt

VERSIONFILE="_build/version.txt"
NEWVERSIONFILE="_build/version_new.txt"

REMOTE_USER="root"
REMOTE_HOST="manage"  
# REMOTE_DIR="$DEPLOY_DIR"


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
ssh -i ~/Downloads/ssh_keys/enyim $REMOTE_USER@$REMOTE_HOST "mkdir -p $REMOTE_DIR"

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
  scp -i ~/Downloads/ssh_keys/enyim "${OUT_PATH}" $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/${BIN_NAME}

  echo "Create .md5 file..."
  # md5sum "${OUT_PATH}" > "${OUT_PATH}.md5"
    (
    cd "${BUILD_DIR}"
    md5sum "${BIN_NAME}" > "${BIN_NAME}.md5"
  )
  echo "Uploading .md5 file..."
  scp -i ~/Downloads/ssh_keys/enyim "${OUT_PATH}.md5" $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/${BIN_NAME}.md5


done

echo "==> Done. Binaries are in: ${BUILD_DIR}"
echo "    Version written to: ${NEWVERSIONFILE}"
mv ${NEWVERSIONFILE} ${VERSIONFILE}




# VERSION="$1"
# SSH_TARGET="$2"
# REMOTE_ROOT_DIR="$3"
# LOCAL_ROOT_DIR="${4:-./dist}"

# LOCAL_DIR="${LOCAL_ROOT_DIR}/${VERSION}"
# REMOTE_DIR="${REMOTE_ROOT_DIR%/}/${VERSION}"

# if [[ ! -d "${LOCAL_DIR}" ]]; then
#   echo "Local build dir not found: ${LOCAL_DIR}" >&2
#   exit 1
# fi

# echo "==> Uploading RunGuard CLI ${VERSION}"
# echo "    Local dir : ${LOCAL_DIR}"
# echo "    Remote dir: ${SSH_TARGET}:${REMOTE_DIR}"

# ssh "${SSH_TARGET}" "mkdir -p '${REMOTE_DIR}'"

# for f in "${LOCAL_DIR}"/runguard-*; do
#   if [[ ! -f "${f}" ]]; then
#     echo "No files matching ${LOCAL_DIR}/runguard-* found" >&2
#     exit 1
#   fi

#   base="$(basename "${f}")"
#   md5_file="${base}.md5"

#   echo "  -> ${base}"

#   # Create .md5 locally
#   (
#     cd "${LOCAL_DIR}"
#     md5sum "${base}" > "${md5_file}"
#   )

#   # Upload binary + .md5
#   scp "${LOCAL_DIR}/${base}" "${LOCAL_DIR}/${md5_file}" \
#     "${SSH_TARGET}:${REMOTE_DIR}/"
# done

# echo "==> Upload complete:"
# echo "    https://downloads.runguard.smega.eu/${VERSION}/"
