#!/usr/bin/env bash
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"

cleanup() {
  echo ""
  echo "Stopping..."
  kill "$BACKEND_PID" "$FRONTEND_PID" 2>/dev/null
  wait "$BACKEND_PID" "$FRONTEND_PID" 2>/dev/null
}
trap cleanup INT TERM

echo "Starting backend on :8080..."
cd "$ROOT/backend"
go run . &
BACKEND_PID=$!

echo "Starting frontend on :3000..."
cd "$ROOT/frontend"
npm run dev -- --port 3000 &
FRONTEND_PID=$!

wait "$BACKEND_PID" "$FRONTEND_PID"
