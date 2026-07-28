#!/usr/bin/env bash
set -euo pipefail

# Ad-hoc/cron-able MongoDB backup for RunGuard.
#
# This is a code-side complement to (not a replacement for) configuring
# real continuous backups in Atlas's dashboard — Atlas backups are the
# actual safety net; this script is useful for quick manual snapshots
# before a risky migration/deploy, or as a cheap cron job if you want an
# extra off-Atlas copy.
#
# Usage:
#   MONGO_URI="mongodb+srv://user:pass@host/runguard" ./scripts/backup_mongo.sh
#   ./scripts/backup_mongo.sh --config ../backend/config.yml
#
# Requires `mongodump` (part of the MongoDB Database Tools) on PATH.
#
# Env vars:
#   MONGO_URI    - full connection string (overrides --config)
#   BACKUP_DIR   - where archives are written (default: ./backups)
#   KEEP_DAYS    - delete local archives older than this many days (default: 14)

BACKUP_DIR="${BACKUP_DIR:-./backups}"
KEEP_DAYS="${KEEP_DAYS:-14}"
CONFIG_FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --config)
      CONFIG_FILE="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if ! command -v mongodump >/dev/null 2>&1; then
  echo "mongodump not found. Install the MongoDB Database Tools first:" >&2
  echo "  https://www.mongodb.com/docs/database-tools/installation/" >&2
  exit 1
fi

if [[ -z "${MONGO_URI:-}" ]]; then
  if [[ -z "$CONFIG_FILE" ]]; then
    CONFIG_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/backend/config.yml"
  fi
  if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "No MONGO_URI set and config file not found at: $CONFIG_FILE" >&2
    echo "Set MONGO_URI directly, or pass --config path/to/config.yml" >&2
    exit 1
  fi
  # Minimal YAML scrape — good enough for this one nested field, avoids
  # requiring yq/python just to read a connection string.
  MONGO_URI="$(awk '/^mongo:/{f=1} f && /uri:/{gsub(/.*uri:[[:space:]]*"?/,""); gsub(/"[[:space:]]*$/,""); print; exit}' "$CONFIG_FILE")"
  if [[ -z "$MONGO_URI" ]]; then
    echo "Could not extract mongo.uri from $CONFIG_FILE" >&2
    exit 1
  fi
fi

mkdir -p "$BACKUP_DIR"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARCHIVE="$BACKUP_DIR/runguard_${STAMP}.archive.gz"

echo "==> Backing up to $ARCHIVE"
mongodump --uri="$MONGO_URI" --archive="$ARCHIVE" --gzip

echo "==> Pruning archives older than ${KEEP_DAYS} days in $BACKUP_DIR"
find "$BACKUP_DIR" -name 'runguard_*.archive.gz' -mtime "+${KEEP_DAYS}" -print -delete

echo "==> Done: $ARCHIVE"
