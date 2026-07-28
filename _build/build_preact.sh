#!/usr/bin/env bash

set -euo pipefail



BUILD_DIR="./tmp_frontend"
REMOTE_USER="root"
REMOTE_HOST="manage"  
REMOTE_DIR="/srv/runguard/frontend"



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
rsync -rav -e "ssh -i ~/Downloads/ssh_keys/enyim" --delete ${BUILD_DIR}/dist/ $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/

# reload nginx
echo "Reloading nginx..."
ssh -i ~/Downloads/ssh_keys/enyim $REMOTE_USER@$REMOTE_HOST 'systemctl reload nginx'

# cleanup
echo "Cleaning up..."
rm -fr  "$BUILD_DIR"
