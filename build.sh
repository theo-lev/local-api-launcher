#!/usr/bin/env bash
set -e

ROOT="$(cd "$(dirname "$0")" && pwd)"
OUT="$ROOT/dist"
mkdir -p "$OUT"

echo "==> Building frontend..."
cd "$ROOT/frontend"
npm install
npm run build

echo "==> Cross-compiling backend..."
cd "$ROOT/backend"

targets=(
  "darwin  amd64 api-manager-darwin-amd64"
  "darwin  arm64 api-manager-darwin-arm64"
  "linux   amd64 api-manager-linux-amd64"
  "linux   arm64 api-manager-linux-arm64"
  "windows amd64 api-manager-windows-amd64.exe"
)

for entry in "${targets[@]}"; do
  read -r os arch name <<< "$entry"
  echo "   $os/$arch -> dist/$name"
  GOOS=$os GOARCH=$arch go build -o "$OUT/$name" .
done

echo ""
echo "Done! Binaries are in dist/"
echo "  macOS (Intel):   ./dist/api-manager-darwin-amd64"
echo "  macOS (Apple):   ./dist/api-manager-darwin-arm64"
echo "  Linux (x64):     ./dist/api-manager-linux-amd64"
echo "  Linux (ARM64):   ./dist/api-manager-linux-arm64"
echo "  Windows (x64):   dist\\api-manager-windows-amd64.exe"
echo ""
echo "Run the binary and open the URL it prints (http://127.0.0.1:9000 by default)"
