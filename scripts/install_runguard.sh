#!/usr/bin/env bash
set -euo pipefail

# No defaults baked into this script on purpose — it's served from your own
# RunGuard instance (or run standalone), so it has to be told where that
# instance actually is via env vars or args. If you're distributing this
# from your own instance, template/sed these in at deploy time, or just
# always set RUNGUARD_DOWNLOAD_BASE / RUNGUARD_VERSIONS_API.
DEFAULT_DOWNLOAD_BASE="${RUNGUARD_DOWNLOAD_BASE:-}"
VERSIONS_API_URL_DEFAULT="${RUNGUARD_VERSIONS_API:-}"

if [[ -z "${VERSIONS_API_URL_DEFAULT}" ]]; then
  echo "ERROR: set RUNGUARD_VERSIONS_API (e.g. https://your-domain.example/api/public/cli/versions)" >&2
  exit 1
fi

# Usage:
#   RUNGUARD_VERSIONS_API=... RUNGUARD_DOWNLOAD_BASE=... bash install.sh [version-or-channel] [download_base]
#
# version-or-channel:
#   - latest  (default)
#   - stable
#   - rc
#   - nightly
#   - or explicit semver: 0.1.3
#
# If download_base is omitted:
#   -> script uses download_base from the API when available,
#      otherwise falls back to RUNGUARD_DOWNLOAD_BASE (if set).
#
# Install location selection:
#   1) $RUNGUARD_INSTALL_DIR (if set)
#   2) /usr/local/bin        (sudo if needed)
#   3) $HOME/.local/bin
#   4) $HOME/bin
#
# Examples (substitute your own RunGuard instance's domain):
#   curl -fsSL https://your-domain.example/install.sh | RUNGUARD_VERSIONS_API=https://your-domain.example/api/public/cli/versions bash
#   curl -fsSL https://your-domain.example/install.sh | RUNGUARD_VERSIONS_API=https://your-domain.example/api/public/cli/versions bash -s -- stable

VERSION_ARG="${1:-latest}"

# Track if the user explicitly provided DOWNLOAD_BASE
USER_PROVIDED_DOWNLOAD_BASE=0
if [[ $# -ge 2 && -n "${2:-}" ]]; then
  USER_PROVIDED_DOWNLOAD_BASE=1
fi
DOWNLOAD_BASE="${2:-$DEFAULT_DOWNLOAD_BASE}"

is_channel() {
  case "$1" in
    latest|stable|rc|nightly) return 0 ;;
    *) return 1 ;;
  esac
}

detect_version_for_channel() {
  local api_url="$1"
  local channel="$2"
  local json

  echo "==> Resolving channel '${channel}' from API: ${api_url}" >&2

  if ! json="$(curl -fsSL "${api_url}" 2>/dev/null)"; then
    echo "ERROR: Failed to fetch version info from ${api_url}" >&2
    return 1
  fi

  # Decide which JSON field to look for
  local field
  case "${channel}" in
    latest)  field="latest" ;;
    stable)  field="stable" ;;
    rc)      field="rc" ;;
    nightly) field="nightly" ;;
    *)       field="latest" ;;  # should not happen
  esac

  # Extract desired field; if empty, fall back to "latest"
  local version
  version="$(printf '%s\n' "$json" \
    | sed -n "s/.*\"${field}\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" \
    | head -n1)"

  if [[ -z "${version}" ]]; then
    echo "    Channel '${channel}' not present, falling back to 'latest'." >&2
    version="$(printf '%s\n' "$json" \
      | sed -n 's/.*"latest"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
      | head -n1)"
  fi

  if [[ -z "${version}" ]]; then
    echo "ERROR: Could not resolve version for channel '${channel}'" >&2
    return 1
  fi

  # If user did NOT override the download base, take it from the API if present
  if [[ "${USER_PROVIDED_DOWNLOAD_BASE}" -eq 0 ]]; then
    local api_base
    api_base="$(printf '%s\n' "$json" \
      | sed -n 's/.*"download_base"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
      | head -n1)"
    if [[ -n "${api_base}" ]]; then
      DOWNLOAD_BASE="${api_base}"
    fi
  fi

  echo "    Resolved version      : ${version}" >&2
  echo "    Download base         : ${DOWNLOAD_BASE}" >&2

  printf '%s\n' "${version}"
}

choose_install_dir() {
  # 1) explicit env
  if [[ -n "${RUNGUARD_INSTALL_DIR:-}" ]]; then
    printf '%s\n' "${RUNGUARD_INSTALL_DIR}"
    return 0
  fi

  # 2) /usr/local/bin if writable
  if [[ -d "/usr/local/bin" && -w "/usr/local/bin" ]]; then
    printf '%s\n' "/usr/local/bin"
    return 0
  fi

  # 3) ~/.local/bin and ~/bin (no sudo)
  if [[ -n "${HOME:-}" ]]; then
    if [[ -d "${HOME}/.local/bin" || ! -e "${HOME}/.local/bin" ]]; then
      printf '%s\n' "${HOME}/.local/bin"
      return 0
    fi
    if [[ -d "${HOME}/bin" || ! -e "${HOME}/bin" ]]; then
      printf '%s\n' "${HOME}/bin"
      return 0
    fi
  fi

  # 4) fall back to /usr/local/bin (sudo may be required)
  printf '%s\n' "/usr/local/bin"
}

echo "==> RunGuard CLI installer"

# Resolve version
if is_channel "${VERSION_ARG}"; then
  CHANNEL="${VERSION_ARG}"
  VERSION="$(detect_version_for_channel "${VERSIONS_API_URL_DEFAULT}" "${CHANNEL}")"
else
  VERSION="${VERSION_ARG}"
  echo "==> Using requested version: ${VERSION}" >&2
  echo "    Download base         : ${DOWNLOAD_BASE}" >&2
fi

echo "    Version               : ${VERSION}"
echo "    Final download base   : ${DOWNLOAD_BASE}"

# Detect platform
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "${OS}" in
  linux)   goos="linux" ;;
  darwin)  goos="darwin" ;;
  *)
    echo "Unsupported OS: ${OS}" >&2
    exit 1
    ;;
esac

case "${ARCH}" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *)
    echo "Unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

BIN_NAME="runguard-${goos}-${goarch}-${VERSION}"
REMOTE_BIN_URL="${DOWNLOAD_BASE%/}/${VERSION}/${BIN_NAME}"
REMOTE_SHA256_URL="${REMOTE_BIN_URL}.sha256"

INSTALL_DIR="$(choose_install_dir)"
INSTALL_PATH="${INSTALL_DIR}/runguard"
TMP_BIN="$(mktemp /tmp/runguard.XXXXXX)"

echo "    Detected OS/arch      : ${goos}/${goarch}"
echo "    Downloading from      : ${REMOTE_BIN_URL}"
echo "    Install directory     : ${INSTALL_DIR}"

# Download binary
if ! curl -fsSL "${REMOTE_BIN_URL}" -o "${TMP_BIN}"; then
  echo "ERROR: Failed to download ${REMOTE_BIN_URL}" >&2
  rm -f "${TMP_BIN}"
  exit 1
fi
chmod +x "${TMP_BIN}"

# Verify checksum. This is fatal on mismatch (or if no checksum can be
# obtained/computed at all) — a check that only warns isn't actually
# protecting against a tampered or corrupted binary.
echo "==> Verifying SHA256 checksum..." >&2
TMP_SHA256="$(mktemp /tmp/runguard.sha256.XXXXXX)"

if ! curl -fsSL "${REMOTE_SHA256_URL}" -o "${TMP_SHA256}" 2>/dev/null; then
  echo "ERROR: No .sha256 file found at ${REMOTE_SHA256_URL}; refusing to install without a checksum." >&2
  rm -f "${TMP_BIN}" "${TMP_SHA256}"
  exit 1
fi

expected="$(awk '{print $1}' "${TMP_SHA256}" 2>/dev/null || true)"
actual=""

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${TMP_BIN}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "${TMP_BIN}" | awk '{print $1}')"
else
  echo "ERROR: Neither sha256sum nor shasum found; cannot verify checksum, refusing to install." >&2
  rm -f "${TMP_BIN}" "${TMP_SHA256}"
  exit 1
fi

if [[ -z "${expected}" || -z "${actual}" || "${expected}" != "${actual}" ]]; then
  echo "ERROR: Checksum mismatch! (expected ${expected:-<none>}, got ${actual:-<none>})" >&2
  echo "       The download may be corrupted or tampered with. Not installing." >&2
  rm -f "${TMP_BIN}" "${TMP_SHA256}"
  exit 1
fi
echo "    Checksum OK." >&2
rm -f "${TMP_SHA256}"

echo "==> Installing to ${INSTALL_PATH}"

mkdir -p "${INSTALL_DIR}"
if [[ -w "${INSTALL_DIR}" ]]; then
  mv "${TMP_BIN}" "${INSTALL_PATH}"
else
  # use sudo only for system paths
  case "${INSTALL_DIR}" in
    /usr/*|/opt/*)
      echo "    ${INSTALL_DIR} not writable, using sudo..." >&2
      sudo mv "${TMP_BIN}" "${INSTALL_PATH}"
      ;;
    *)
      echo "ERROR: ${INSTALL_DIR} is not writable and sudo will not be used." >&2
      echo "       Set RUNGUARD_INSTALL_DIR or run this script with a writable target." >&2
      rm -f "${TMP_BIN}"
      exit 1
      ;;
  esac
fi

echo "    Setting executable bit..."
chmod +x "${INSTALL_PATH}" 2>/dev/null || sudo chmod +x "${INSTALL_PATH}"

echo "==> Installed:"
echo "    $(command -v runguard || echo "${INSTALL_PATH}")"
echo
echo "You can now wrap a job, for example:"
echo
echo "  runguard wrap --key=YOUR_HEARTBEAT_KEY -- /opt/scripts/orders.sh"
echo
