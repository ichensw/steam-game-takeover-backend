#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if ! command -v go >/dev/null 2>&1; then
  echo "[ERROR] Go was not found in PATH."
  echo "Install Go or add it to PATH, then run this script again."
  exit 1
fi

mkdir -p bin

echo "[INFO] Building steam-game-takeover-backend..."
GOOS=windows GOARCH=amd64 go build -o "bin/steam-game-takeover-backend.exe" "./cmd/server"

echo "[OK] Built: $(pwd)/bin/steam-game-takeover-backend.exe"
