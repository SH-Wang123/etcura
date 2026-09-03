#!/usr/bin/env bash
# Build + sign the macOS app. Works around the codesign failure
# "resource fork, Finder information, or similar detritus not allowed",
# which appears when the project directory lives in an iCloud-synced
# folder (file provider re-tags files between build and signing).
#
# wails build exits non-zero when its internal signing step fails even
# though compile+package succeeded, so the exit code is ignored and the
# signing is redone here.
#
# Usage:
#   ./scripts/build.sh              # arm64 (Apple Silicon)
#   ./scripts/build.sh amd64        # Intel
#   ./scripts/build.sh universal    # Universal binary
set -euo pipefail

cd "$(dirname "$0")/.."

ARCH="${1:-arm64}"
export PATH="$PATH:$HOME/go/bin"

APP="build/bin/etcd-eye.app"

echo "==> wails build -platform darwin/${ARCH}"
wails build -platform "darwin/${ARCH}" || echo "    wails reported a signing error; re-signing here"

test -d "$APP" || { echo "FATAL: $APP was not produced"; exit 1; }

echo "==> stripping extended attributes"
xattr -cr "$APP" || true

echo "==> ad-hoc signing"
if ! codesign --force --deep --sign - "$APP" 2>/dev/null; then
  echo "    direct signing failed (file provider tagging); signing via /tmp"
  rm -rf /tmp/etcd-eye-sign.app
  cp -R "$APP" /tmp/etcd-eye-sign.app
  xattr -cr /tmp/etcd-eye-sign.app
  codesign --force --deep --sign - /tmp/etcd-eye-sign.app
  # Verify while the bundle is still outside the iCloud-synced tree: file
  # provider re-tags files inside ~/Documents after copy-back, which makes a
  # later strict verify report "detritus" even though the app runs fine.
  codesign --verify --deep --strict /tmp/etcd-eye-sign.app
  rm -rf "$APP"
  cp -R /tmp/etcd-eye-sign.app "$APP"
  rm -rf /tmp/etcd-eye-sign.app
fi

echo "==> OK: $APP ($(file -b "$APP/Contents/MacOS/etcd-eye" | cut -d, -f1-2))"
echo "    it: xattr -cr ${APP} && codesign --force --deep --sign - ${APP}"
