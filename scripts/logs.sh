#!/usr/bin/env bash
# Follow both log sources at once: the daemon's journal (it runs as a systemd
# user unit) and the Lomiri user journal (app start-up, QML errors, apparmor
# denials surfacing through the launcher, indicator-sound complaints).
# Usage: scripts/logs.sh [journal|daemon|all] [grep-pattern]
#
# The host has two adb devices attached; pick one via ADB_SERIAL=3b2ad391.
set -uo pipefail

WHAT="${1:-all}"
FILTER="${2:-}"
FALLBACK_LOG='$HOME/.cache/waymedia/waymediad.log'

ADB=(adb)
[ -n "${ADB_SERIAL:-}" ] && ADB=(adb -s "$ADB_SERIAL")

journal="journalctl --user -n 50 -f"
[ -n "$FILTER" ] && journal="$journal | grep --line-buffered -i '$FILTER'"
journal="$journal | sed -u 's/^/[journal] /'"

# systemd path first; the setsid fallback in run.sh logs to a file instead.
daemon="journalctl --user -u waymediad -n 100 -f"
[ -n "$FILTER" ] && daemon="$daemon | grep --line-buffered -i '$FILTER'"
daemon="$daemon | sed -u 's/^/[daemon]  /'"
daemon="{ $daemon & mkdir -p \$(dirname $FALLBACK_LOG); touch $FALLBACK_LOG; \
          tail -n 50 -F $FALLBACK_LOG | sed -u 's/^/[daemon]  /' ; }"

case "$WHAT" in
    journal) cmd="$journal" ;;
    daemon)  cmd="$daemon" ;;
    all)     cmd="{ $journal & $daemon ; }" ;;
    *) echo "usage: $0 [journal|daemon|all] [grep-pattern]" >&2; exit 2 ;;
esac

exec "${ADB[@]}" shell "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/32011/bus $cmd"
