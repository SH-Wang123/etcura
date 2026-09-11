#!/usr/bin/env bash
# Cross-compile the Windows build from macOS/Linux.
#
# Requires mingw-w64 for the WebView2 loader:
#   brew install mingw-w64          # macOS
#   sudo apt install mingw-w64      # Debian/Ubuntu
#
# Usage:
#   ./scripts/build-windows.sh              # amd64 (Windows 10/11 x64)
#   ./scripts/build-windows.sh arm64        # Windows on ARM
set -euo pipefail

cd "$(dirname "$0")/.."
export PATH="$PATH:$HOME/go/bin"

ARCH="${1:-amd64}"
EXE="build/bin/etcura.exe"

case "$ARCH" in
  amd64) CC=x86_64-w64-mingw32-gcc ;;
  arm64) CC=aarch64-w64-mingw32-gcc ;;
  *) echo "unsupported arch: $ARCH (use amd64 or arm64)"; exit 1 ;;
esac

if ! command -v "$CC" >/dev/null 2>&1; then
  echo "FATAL: $CC not found. Install mingw-w64 (brew install mingw-w64)."
  exit 1
fi

echo "==> wails build -platform windows/$ARCH (webview2 embed)"
# -webview2 embed bundles the WebView2 bootstrapper so the app can install the
# runtime itself when the target machine lacks it (offline-capable).
wails build -platform "windows/$ARCH" -webview2 embed

test -f "$EXE" || { echo "FATAL: $EXE was not produced"; exit 1; }

# Windows executables cannot be codesigned on macOS; publish a zip instead.
ZIP="build/bin/etcura-windows-$ARCH.zip"
rm -f "$ZIP"
(cd build/bin && zip -q "etcura-windows-$ARCH.zip" etcura.exe)

echo "==> OK: $EXE ($(file -b "$EXE" | cut -d, -f1))"
echo "    package: $ZIP"
