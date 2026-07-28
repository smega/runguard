#!/usr/bin/env bash

set -euo pipefail

#######################################
# Args & env selection
#######################################
# if [[ $# -ne 1 ]]; then
#   echo "Usage: $0 {prod|nonprod}" >&2
#   exit 1
# fi

# ENV="$1"

# case "$ENV" in
#   prod)
#     DOMAIN="vpsnap.com"
#     DEPLOY_DIR="/srv/prod/vpsnap"
#     PORT="7070"
#     ;;
#   nonprod)
#     DOMAIN="vpsnap.smega.eu"
#     DEPLOY_DIR="/srv/nonprod/vpsnap"
#     PORT="8081"
#     ;;
#   *)
#     echo "Invalid environment: $ENV" >&2
#     echo "Use: prod or nonprod" >&2
#     exit 1
#     ;;
# esac

BUILD_DIR="./tmp_backend"
REMOTE_USER="root"
REMOTE_HOST="manage"  
# REMOTE_DIR="$DEPLOY_DIR"
REMOTE_DIR="/srv/runguard/backend"

echo "Deploying backend"
# echo "Deploying to ENV=$ENV"
# echo "  DOMAIN     = $DOMAIN"
# echo "  DEPLOY_DIR = $DEPLOY_DIR"
# echo "  REMOTE     = $REMOTE_USER@$REMOTE_HOST"


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

# modify config for cloud deployment
# echo "Modifying configuration for cloud deployment..."
# yq -i '.mysql.host = "db.smega.eu"' "$BUILD_DIR/config.yml"
# yq -i '.env = "prod"' "$BUILD_DIR/config.yml"
# yq -i '.mailgun.debug = false' "$BUILD_DIR/config.yml"
# yq -i '.billing.paypal.return_url = "https://'"$DOMAIN"'/billing?paypal=1"' "$BUILD_DIR/config.yml"
# yq -i '.billing.paypal.cancel_url = "https://'"$DOMAIN"'/billing?paypal=cancel"' "$BUILD_DIR/config.yml"
# yq -i '.cors.allowed_origins = ["https://'"$DOMAIN"'"]' "$BUILD_DIR/config.yml"
# yq -i '.mailgun.web_base_url = "https://'"$DOMAIN"'"' "$BUILD_DIR/config.yml"
# yq -i '.alerts.from = "VPSnap Alerts '"$DOMAIN"'"' "$BUILD_DIR/config.yml"

# if [[ "$ENV" == "prod" ]]; then
#   yq -i '.server.addr = ":7070"' "$BUILD_DIR/config.yml"
#   yq -i '.mysql.database = "vpsnap_prod"' "$BUILD_DIR/config.yml"
#   yq -i '.billing.paypal.sandbox = false' "$BUILD_DIR/config.yml"
# # else
# #   yq -i '.server.addr = ":8081"' "$BUILD_DIR/config.yml"
# fi

# Create remote dir if it doesn't exist
echo "Ensuring remote deployment directory exists..."
ssh -i ~/Downloads/ssh_keys/enyim $REMOTE_USER@$REMOTE_HOST "mkdir -p $REMOTE_DIR"

# upload config file
echo "Uploading configuration file..."
scp -i ~/Downloads/ssh_keys/enyim "$BUILD_DIR/config.yml" $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/config.yml
# build backend
echo "Building backend..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go -C "$BUILD_DIR" build -v -ldflags="-s -w" -o ../runguard
echo "Build completed."

# stop existing service
echo "Stopping existing runguard..."
ssh -i ~/Downloads/ssh_keys/enyim $REMOTE_USER@$REMOTE_HOST "systemctl stop runguard.service"

# upload backend binary
echo "Uploading new runguard binary..."
scp -i ~/Downloads/ssh_keys/enyim runguard $REMOTE_USER@$REMOTE_HOST:$REMOTE_DIR/runguard
ssh -i ~/Downloads/ssh_keys/enyim $REMOTE_USER@$REMOTE_HOST "chmod +x $REMOTE_DIR/runguard"

# start service
echo "Starting runguard..."
ssh -i ~/Downloads/ssh_keys/enyim $REMOTE_USER@$REMOTE_HOST "systemctl daemon-reload && systemctl enable --now runguard.service"
# cleanup
echo "Cleaning up..."
rm -fr "$BUILD_DIR"
rm runguard