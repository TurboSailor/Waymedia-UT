#!/usr/bin/env bash
# Install a built .click on the phone, verify the click/apparmor registration and
# restart the daemon that caches a path into the old unpack directory.
#
# Usage: WAYMEDIA_SUDO_PASS=<pass> scripts/deploy.sh [CLICK]
#   click install needs sudo, and every adb shell is a fresh session, so the
#   password cannot be cached — it is read from the environment (or a local,
#   untracked scripts/deploy.env) and never stored in the repository.
#
# With several adb devices attached, bare adb calls fail with "more than one
# device", so pick one: ADB_SERIAL=<serial> scripts/deploy.sh
set -euo pipefail

CLICK="${1:-}"
# Optional untracked file for local convenience: WAYMEDIA_SUDO_PASS=...
[ -f scripts/deploy.env ] && . scripts/deploy.env
PASS="${WAYMEDIA_SUDO_PASS:-}"
[ -n "$PASS" ] || { echo "deploy.sh: set WAYMEDIA_SUDO_PASS (or scripts/deploy.env)" >&2; exit 1; }
REMOTE_DIR="/home/phablet/Downloads"
PKG="waymedia.turbosailor"
SESSION_BUS="unix:path=/run/user/32011/bus"

ADB=(adb)
[ -n "${ADB_SERIAL:-}" ] && ADB=(adb -s "$ADB_SERIAL")

die() { echo "deploy.sh: $*" >&2; exit 1; }
# Every adb shell is a fresh session, so sudo never has a cached credential.
sudo_sh() { "${ADB[@]}" shell "echo '$PASS' | sudo -S $1 2>&1" | tr -d '\r'; }
user_sh() { "${ADB[@]}" shell "DBUS_SESSION_BUS_ADDRESS=$SESSION_BUS $1 2>&1" | tr -d '\r'; }

if [ -z "$CLICK" ]; then
    CLICK="$(ls -t build/*.click 2>/dev/null | head -n1 || true)"
    [ -n "$CLICK" ] || die "no .click in build/ (run: make click)"
fi
[ -f "$CLICK" ] || die "$CLICK not found"
"${ADB[@]}" get-state >/dev/null 2>&1 || die "no adb device"

base="$(basename "$CLICK")"
echo ">> pushing $base"
"${ADB[@]}" push "$CLICK" "$REMOTE_DIR/$base" >/dev/null

echo ">> click install"
# debsig signatures are absent for locally built packages -> allow-unauthenticated.
sudo_sh "click install --force --allow-unauthenticated --user=phablet $REMOTE_DIR/$base"

pkg="$("${ADB[@]}" shell "click list 2>/dev/null" | tr -d '\r' | sed -n "s/^$PKG\t.*/&/p")"
if [ -z "$pkg" ]; then
    echo "!! $PKG is not in click list; leftovers:"
    sudo_sh "ls -d /opt/click.ubuntu.com/$PKG 2>/dev/null"
    die "install failed (clean up with: sudo rm -rf /opt/click.ubuntu.com/$PKG)"
fi
echo ">> click list: $pkg"

prof="$(sudo_sh "aa-status" | sed -n "/${PKG//./\\.}/p" | head -n3)"
if [ -z "$prof" ]; then
    echo ">> apparmor profile missing, regenerating hooks"
    sudo_sh "aa-clickhook -f"
    prof="$(sudo_sh "aa-status" | sed -n "/${PKG//./\\.}/p" | head -n3)"
fi
[ -n "$prof" ] || die "apparmor profile for $PKG was not generated"
echo ">> aa-status:"
echo "$prof"

# The daemon runs from .../current/bin/waymediad; the symlink now points at the
# fresh unpack directory but the running process still holds the old inode, so
# it would keep serving the previous build (and its MPRIS name) forever.
echo ">> restarting waymediad (user unit)"
if [ -n "$(user_sh "systemctl --user cat waymediad.service >/dev/null && echo yes")" ]; then
    user_sh "systemctl --user daemon-reload" >/dev/null
    user_sh "systemctl --user restart waymediad.service" >/dev/null \
        || echo "!! could not restart waymediad"
    user_sh "systemctl --user --no-pager --lines=0 status waymediad.service" | head -n5
else
    echo ">> waymediad.service not installed yet; the first app launch installs it"
fi

ver="$(printf '%s' "$pkg" | awk '{print $2}')"
echo ">> app id: ${PKG}_waymedia_$ver  (tap the icon; adb launches lack a trust session)"
