#!/usr/bin/env bash

set -euo pipefail

# Config is overridable via env vars so this script works unchanged from a
# developer's machine or from CI (see .github/workflows/deploy.yml). See
# deploy.env.example for a one-time way to set these instead of exporting
# them in every shell.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ -f "${SCRIPT_DIR}/deploy.env" ] && source "${SCRIPT_DIR}/deploy.env"

BUILD_DIR="./tmp_backend"
REMOTE_USER="${REMOTE_USER:-root}"
REMOTE_HOST="${REMOTE_HOST:?set REMOTE_HOST (env var or _build/deploy.env)}"
REMOTE_DIR="${REMOTE_DIR:-/srv/runguard/backend}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519}"

echo "Deploying backend to ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}"


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
rsync -ra --delete ./backend/ "$BUILD_DIR/"

# Create remote dir if it doesn't exist
echo "Ensuring remote deployment directory exists..."
ssh -i "$SSH_KEY" "$REMOTE_USER@$REMOTE_HOST" "mkdir -p $REMOTE_DIR"

# upload config file (config.yml is never in git — it must already exist
# locally, e.g. copied from config.sample.yml and filled in)
echo "Uploading configuration file..."
scp -i "$SSH_KEY" "$BUILD_DIR/config.yml" "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/config.yml"

# build backend
echo "Building backend..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go -C "$BUILD_DIR" build -v -ldflags="-s -w" -o ../runguard
echo "Build completed."

# stop existing service
echo "Stopping existing runguard..."
ssh -i "$SSH_KEY" "$REMOTE_USER@$REMOTE_HOST" "systemctl stop runguard.service"

# upload backend binary
echo "Uploading new runguard binary..."
scp -i "$SSH_KEY" runguard "$REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/runguard"
ssh -i "$SSH_KEY" "$REMOTE_USER@$REMOTE_HOST" "chmod +x $REMOTE_DIR/runguard"

# start service
echo "Starting runguard..."
ssh -i "$SSH_KEY" "$REMOTE_USER@$REMOTE_HOST" "systemctl daemon-reload && systemctl enable --now runguard.service"

# cleanup
echo "Cleaning up..."
rm -fr "$BUILD_DIR"
rm runguard