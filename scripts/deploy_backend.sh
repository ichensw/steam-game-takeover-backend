#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

DEPLOY_HOST="${DEPLOY_HOST:-47.102.200.211}"
DEPLOY_USER="${DEPLOY_USER:-root}"
APP_DIR="${APP_DIR:-/opt/steam-game-takeover-backend}"
SERVICE_NAME="${SERVICE_NAME:-steam-game-takeover-backend.service}"
PUBLIC_HEALTH_URL="${PUBLIC_HEALTH_URL:-https://rabbits.ink/miniprogram-api/api/health}"
REMOTE_MYSQL_CMD="${REMOTE_MYSQL_CMD:-mysql -uroot steam_takeover}"
MIGRATIONS="${MIGRATIONS:-}"
DEPLOY_SSH_OPTS="${DEPLOY_SSH_OPTS:-}"

REMOTE="${DEPLOY_USER}@${DEPLOY_HOST}"
BINARY_NAME="steam-game-takeover-backend"

if [[ -n "$DEPLOY_SSH_OPTS" ]]; then
  # shellcheck disable=SC2206
  SSH_ARGS=($DEPLOY_SSH_OPTS)
else
  SSH_ARGS=(-o StrictHostKeyChecking=accept-new)
fi

log() {
  printf '[%s] %s\n' "$1" "$2"
}

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    log ERROR "$1 was not found in PATH."
    exit 1
  fi
}

ssh_remote() {
  ssh "${SSH_ARGS[@]}" "$REMOTE" "$@"
}

scp_remote() {
  scp "${SSH_ARGS[@]}" "$@"
}

status() {
  log INFO "Remote: $REMOTE"
  ssh_remote bash -s -- "$APP_DIR" "$SERVICE_NAME" <<'REMOTE_SCRIPT'
set -u
APP_DIR="$1"
SERVICE_NAME="$2"

echo "[INFO] Service active:"
systemctl is-active "$SERVICE_NAME" || true

echo
echo "[INFO] Service status:"
systemctl --no-pager --lines=8 status "$SERVICE_NAME" || true

echo
echo "[INFO] Binary:"
ls -lh "$APP_DIR/steam-game-takeover-backend" 2>/dev/null || true

echo
echo "[INFO] Local health:"
curl -fsS --max-time 8 http://127.0.0.1:8081/api/health
echo
REMOTE_SCRIPT

  echo
  log INFO "Public health: $PUBLIC_HEALTH_URL"
  curl -fsS --max-time 10 "$PUBLIC_HEALTH_URL"
  echo
}

deploy() {
  need go
  need ssh
  need scp
  need curl

  cd "$ROOT_DIR"
  mkdir -p dist

  log INFO "Running tests..."
  go test -count=1 ./...

  log INFO "Building Linux binary..."
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "dist/$BINARY_NAME" ./cmd/server

  stamp="$(date +%Y%m%d%H%M%S)"
  remote_binary="/tmp/$BINARY_NAME.$stamp"
  remote_migration_dir="/tmp/${BINARY_NAME}-migrations.$stamp"
  mysql_cmd_b64="$(printf '%s' "$REMOTE_MYSQL_CMD" | base64 | tr -d '\n')"
  migration_names=()

  log INFO "Uploading binary..."
  scp_remote "dist/$BINARY_NAME" "$REMOTE:$remote_binary"

  if [[ -n "$MIGRATIONS" ]]; then
    log INFO "Uploading selected migrations..."
    ssh_remote "mkdir -p '$remote_migration_dir'"
    for migration in $MIGRATIONS; do
      if [[ ! -f "$migration" ]]; then
        log ERROR "Migration not found: $migration"
        exit 1
      fi
      migration_name="$(basename "$migration")"
      migration_names+=("$migration_name")
      scp_remote "$migration" "$REMOTE:$remote_migration_dir/$migration_name"
    done
  fi

  log INFO "Installing and restarting service..."
  ssh_remote bash -s -- "$APP_DIR" "$SERVICE_NAME" "$remote_binary" "$remote_migration_dir" "$mysql_cmd_b64" "${migration_names[@]}" <<'REMOTE_SCRIPT'
set -euo pipefail
APP_DIR="$1"
SERVICE_NAME="$2"
REMOTE_BINARY="$3"
MIGRATION_DIR="$4"
REMOTE_MYSQL_CMD="$(printf '%s' "$5" | base64 -d)"
shift 5

cd "$APP_DIR"

if [[ "$#" -gt 0 ]]; then
  for migration_name in "$@"; do
    migration_path="$MIGRATION_DIR/$migration_name"
    echo "[INFO] Applying migration: $migration_name"
    eval "$REMOTE_MYSQL_CMD < \"\$migration_path\""
  done
else
  echo "[INFO] No migration selected."
fi

if [[ -f steam-game-takeover-backend ]]; then
  mkdir -p backups
  cp steam-game-takeover-backend "backups/steam-game-takeover-backend.$(date +%Y%m%d%H%M%S).bak"
fi

install -m 0755 "$REMOTE_BINARY" steam-game-takeover-backend
systemctl restart "$SERVICE_NAME"
sleep 2
systemctl is-active --quiet "$SERVICE_NAME"
curl -fsS --max-time 8 http://127.0.0.1:8081/api/health
echo
REMOTE_SCRIPT

  log OK "Backend deployed."
  log INFO "Checking public health..."
  curl -fsS --max-time 10 "$PUBLIC_HEALTH_URL"
  echo
}

usage() {
  cat <<EOF
Usage:
  $0 status
  $0 deploy

Options by environment variable:
  DEPLOY_HOST          default: $DEPLOY_HOST
  DEPLOY_USER          default: $DEPLOY_USER
  APP_DIR              default: $APP_DIR
  SERVICE_NAME         default: $SERVICE_NAME
  PUBLIC_HEALTH_URL    default: $PUBLIC_HEALTH_URL
  DEPLOY_SSH_OPTS      optional ssh/scp options, for example "-i ~/.ssh/id_rsa"
  MIGRATIONS           optional SQL files to apply, for example "migrations/043_x.sql"
  REMOTE_MYSQL_CMD     default: $REMOTE_MYSQL_CMD

No password is stored in this script. ssh/scp/mysql use your machine or server config.
EOF
}

case "${1:-status}" in
  status)
    need ssh
    need curl
    status
    ;;
  deploy)
    deploy
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage
    exit 1
    ;;
esac
