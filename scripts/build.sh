#!/usr/bin/env bash
# Pack the assembled click tree into a .click.
#
# macOS has no `click` tool, so the tree is pushed to the phone, packed there and
# the artefact is pulled back. Usage: scripts/build.sh [PKG_DIR] [OUT_CLICK]
#
# The host has two adb devices attached; bare adb calls fail with "more than one
# device", so pick one: ADB_SERIAL=3b2ad391 scripts/build.sh
set -euo pipefail

PKG_DIR="${1:-build/pkg}"
OUT="${2:-}"
REMOTE="/home/phablet/.cache/waymedia-build"

ADB=(adb)
[ -n "${ADB_SERIAL:-}" ] && ADB=(adb -s "$ADB_SERIAL")

die() { echo "build.sh: $*" >&2; exit 1; }

[ -d "$PKG_DIR" ] || die "$PKG_DIR does not exist (run: make pkg)"
for f in manifest.json waymedia.apparmor waymedia.desktop waymedia.png run.sh \
         waymediad.service bin/waymediad qml/Main.qml; do
    [ -e "$PKG_DIR/$f" ] || die "missing $PKG_DIR/$f"
done
"${ADB[@]}" get-state >/dev/null 2>&1 || die "no adb device (try: adb kill-server && adb start-server)"

echo ">> pushing $PKG_DIR -> $REMOTE/pkg"
"${ADB[@]}" shell "rm -rf $REMOTE && mkdir -p $REMOTE" >/dev/null
"${ADB[@]}" push "$PKG_DIR" "$REMOTE/pkg" >/dev/null

echo ">> click build (on device)"
# adb push does not carry the exec bit reliably; restore it before packing.
"${ADB[@]}" shell "cd $REMOTE && chmod +x pkg/run.sh pkg/bin/* && click build pkg 2>&1" | tr -d '\r'

name="$("${ADB[@]}" shell "cd $REMOTE && ls -1 *.click 2>/dev/null | head -n1" | tr -d '\r\n')"
[ -n "$name" ] || die "click build produced no .click"

[ -n "$OUT" ] || OUT="build/$name"
mkdir -p "$(dirname "$OUT")"
"${ADB[@]}" pull "$REMOTE/$name" "$OUT" >/dev/null
echo ">> $OUT ($(wc -c <"$OUT" | tr -d ' ') bytes)"
