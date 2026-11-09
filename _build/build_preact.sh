#!/usr/bin/env bash

set -euo pipefail



# Config is overridable via env vars so this script works unchanged from a
# developer's machine or from CI (see .github/workflows/deploy.yml). See
# deploy.env.example for a one-time way to set these instead of exporting
# them in every shell.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -f "${SCRIPT_DIR}/deploy.env" ] && source "${SCRIPT_DIR}/deploy.env"

BUILD_DIR="./tmp_frontend"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="${REMOTE_HOST:?set REMOTE_HOST (env var or _build/deploy.env)}"
REMOTE_DIR="${REMOTE_DIR:-/srv/runguard/frontend}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519}"

cleanup() {
  # runs on *any* script exit (success or failure)
  if [[ -n "${BUILD_DIR:-}" && -d "$BUILD_DIR" ]]; then
    echo "Cleaning up temp dir: $BUILD_DIR"
    rm -rf -- "$BUILD_DIR"
  fi
}

trap cleanup EXIT

# prepare tmp dir
echo "Preparing temporary build directory..."
rsync -ra --exclude node_modules ./frontend/ ${BUILD_DIR}

# build frontend
echo "Building frontend..."
cd ${BUILD_DIR}
npm install
npm run build
cd ..

# sync frontend files
echo "Uploading new frontend files..."
rsync -rav -e "ssh -i $SSH_KEY" --delete ${BUILD_DIR}/dist/ "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/"

# reload nginx
echo "Reloading nginx..."
ssh -i "$SSH_KEY" "$REMOTE_USER@$REMOTE_HOST" 'systemctl reload nginx'

# cleanup
echo "Cleaning up..."
rm -fr  "$BUILD_DIR"
