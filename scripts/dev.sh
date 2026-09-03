#!/usr/bin/env bash
# Start `wails dev` reliably in an iCloud-synced project directory.
#
# wails dev compiles AND ad-hoc signs the dev binary on every rebuild. In
# ~/Documents the file provider re-tags fresh files within seconds, so the
# signing step intermittently fails with "resource fork, Finder information,
# or similar detritus not allowed" — and the dev backend then keeps running
# STALE code (hot reload silently dies).
#
# Strategy: run wails dev, watch its log; every time a signing failure is
# detected, strip the xattrs (direct sign, /tmp fallback) and touch a Go file
# to force another rebuild, until the dev server actually serves.
set -uo pipefail

cd "$(dirname "$0")/.."
export PATH="$PATH:$HOME/go/bin"

LOG=/tmp/wails-dev-repair.log
rm -rf build/bin/etcura.app
xattr -cr build/bin 2>/dev/null || true

wails dev > "$LOG" 2>&1 &
WAILS_PID=$!
trap "kill $WAILS_PID 2>/dev/null" EXIT

repair_bundle() {
  local app="build/bin/etcura.app"
  [ -d "$app" ] || return 0
  xattr -cr "$app" 2>/dev/null || true
  if codesign --force --deep --sign - "$app" >/dev/null 2>&1; then
    return 0
  fi
  rm -rf /tmp/etcura-dev-sign.app
  cp -R "$app" /tmp/etcura-dev-sign.app
  xattr -cr /tmp/etcura-dev-sign.app
  codesign --force --deep --sign - /tmp/etcura-dev-sign.app || return 1
  rm -rf "$app"
  cp -R /tmp/etcura-dev-sign.app "$app"
  rm -rf /tmp/etcura-dev-sign.app
  return 0
}

up=0
prevFails=0
for _ in $(seq 1 120); do
  sleep 2
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:34115/ 2>/dev/null)
  if [ "$code" = "200" ]; then up=1; break; fi
  fails=$(grep -ac "codesign failed" "$LOG" 2>/dev/null)
  fails=${fails:-0}
  if [ "$fails" -gt "$prevFails" ]; then
    prevFails=$fails
    echo "signing failure #$fails detected — repairing bundle and forcing rebuild"
    repair_bundle || true
    touch main.go
  fi
done

if [ "$up" = "1" ]; then
  echo "wails dev is up: http://localhost:34115"
else
  echo "FATAL: dev server did not come up; see $LOG"
  exit 1
fi

# Keep watching: later hot-rebuilds can hit the same race; repair on failure.
while kill -0 $WAILS_PID 2>/dev/null; do
  sleep 3
  fails=$(grep -ac "codesign failed" "$LOG" 2>/dev/null)
  fails=${fails:-0}
  if [ "$fails" -gt "$prevFails" ]; then
    prevFails=$fails
    echo "hot-reload signing failure #$fails — repairing and forcing rebuild"
    repair_bundle || true
    touch main.go
  fi
done
