#!/bin/bash
# Click entry point: make sure waymediad is running, then hand over to the UI.
#
# The daemon is installed as a systemd *user* unit rather than being forked from
# here, because the whole point of the app is to keep the MPRIS2 player exported
# while the screen is locked and the UI is gone — Lomiri suspends and eventually
# reaps backgrounded applications, the user manager does not. The UI is a thin
# client and only talks to it over http://127.0.0.1:21980.
set -u

APP_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/waymedia"
UNIT_NAME="waymediad.service"
UNIT_SRC="$APP_DIR/$UNIT_NAME"
UNIT_DIR="$HOME/.config/systemd/user"
UNIT_DST="$UNIT_DIR/$UNIT_NAME"
PIDFILE="$RUNTIME_DIR/waymediad.pid"
LOG="$CACHE_DIR/waymediad.log"
MAX_LOG=5242880

# /proc/net/tcp keeps ports in uppercase hex: 21980 == 0x55DC
# (5*4096 + 5*256 + 13*16 + 12 = 20480 + 1280 + 208 + 12), state 0A == LISTEN.
PORT_HEX=55DC

mkdir -p "$CACHE_DIR" "$RUNTIME_DIR" 2>/dev/null

api_up() {
    grep -q ":$PORT_HEX 00000000:0000 0A" /proc/net/tcp 2>/dev/null
}

wait_for_api() {
    for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
        api_up && return 0
        sleep 0.2
    done
    return 1
}

# ---- systemd path (the normal one) ---------------------------------------
have_systemd_user() {
    command -v systemctl >/dev/null 2>&1 || return 1
    systemctl --user show-environment >/dev/null 2>&1
}

install_unit() {
    mkdir -p "$UNIT_DIR" 2>/dev/null
    # Copy only on change: an unconditional rewrite plus daemon-reload on every
    # launch would restart the bridge (dropping the exported MPRIS name, and with
    # it the lock screen controls) for nothing.
    if ! cmp -s "$UNIT_SRC" "$UNIT_DST"; then
        cp "$UNIT_SRC" "$UNIT_DST" || return 1
        systemctl --user daemon-reload >/dev/null 2>&1
    fi
    return 0
}

# ---- fallback path (no session bus: adb shell, recovery-ish sessions) ------
start_detached() {
    if [ -f "$PIDFILE" ]; then
        pid="$(cat "$PIDFILE" 2>/dev/null)"
        if [ -n "$pid" ] && [ -r "/proc/$pid/comm" ] && grep -q '^waymediad$' "/proc/$pid/comm"; then
            return 0
        fi
    fi
    api_up && return 0

    # Keep the log bounded but truncate in place: a daemon already holding an
    # O_APPEND fd here would otherwise keep writing to the rotated file.
    if [ -f "$LOG" ] && [ "$(stat -c %s "$LOG" 2>/dev/null || echo 0)" -gt "$MAX_LOG" ]; then
        tail -c 262144 "$LOG" >"$LOG.1" 2>/dev/null
        : >"$LOG"
    fi

    echo "=== waymediad start $(date -Is) ===" >>"$LOG"
    # Own session: the daemon must outlive the launching shell (SIGHUP).
    # setsid execs in place, so $! is waymediad itself.
    setsid "$APP_DIR/bin/waymediad" >>"$LOG" 2>&1 </dev/null &
    echo $! >"$PIDFILE"
}

if have_systemd_user && install_unit; then
    # --now covers both "never enabled" and "enabled but stopped"; a running
    # unit is left alone so tapping the icon does not interrupt playback state.
    systemctl --user enable --now "$UNIT_NAME" >/dev/null 2>&1
    api_up || systemctl --user start "$UNIT_NAME" >/dev/null 2>&1
else
    start_detached
fi

# Give the API a moment so the first UI poll does not fail for nothing.
wait_for_api

cd "$APP_DIR"
exec qmlscene "$@" qml/Main.qml
