pragma Singleton
import QtQuick 2.4
import Ubuntu.Components 1.3

// Single source of truth: the whole UI binds to these properties and never
// talks to the network itself. The daemon is the only backend there is
// (http://127.0.0.1:21980), so a daemon that is down degrades to "offline but
// valid" state in exactly one place.
QtObject {
    id: store

    readonly property string base: "http://127.0.0.1:21980"

    // ---- daemon reachability -------------------------------------------
    property bool online: false
    property bool everAnswered: false
    property string lastError: ""

    // ---- payload (null = never loaded, not "empty") ---------------------
    property var status: null

    // A never-loaded status must still render, so `now` falls back to a
    // session-less snapshot with the same shape the daemon sends. Pages can
    // then bind to Store.now.title without guarding every field.
    readonly property var idleNow: ({
        has_session: false,
        "package": "",
        app_name: "",
        status: "Stopped",
        title: "",
        artist: "",
        album: "",
        position_ms: 0,
        can_play: false,
        can_pause: false,
        can_next: false,
        can_previous: false,
        can_stop: false
    })

    readonly property var now: status && status.now ? status.now : idleNow
    readonly property var waydroid: status ? status.waydroid : null
    readonly property var bridge: status ? status.bridge : null
    readonly property string version: status && status.version ? status.version : ""
    readonly property int uptimeSec: status && status.uptime_sec ? status.uptime_sec : 0

    // Local mirror of the daemon settings. Defaults match the daemon's own so
    // the Settings page stays usable (and honest) before the first answer.
    property var settings: ({
        enabled: true,
        poll_ms: 1500,
        idle_poll_ms: 5000
    })
    property bool settingsLoaded: false

    // Which command is in flight, "" when none: the transport keys dim while
    // the daemon dispatches to Android instead of pretending it was instant.
    property string busyAction: ""

    // Toast plumbing as plain state: Main watches the counter, so no
    // Connections object (whose `enabled` is unavailable in QtQuick 2.4) is
    // needed anywhere.
    property string toastText: ""
    property int toastSeq: 0
    function toast(message) {
        toastText = message;
        toastSeq = toastSeq + 1;
    }

    // =====================================================================
    // transport
    // =====================================================================
    // `ok(parsedBody)` on 2xx, `fail(message, httpStatus)` otherwise. A dead
    // daemon shows up as status 0, which every screen renders as an offline
    // state instead of an error popup; 503 means the daemon is fine but the
    // container is not, and it carries its own `error` string.
    function request(method, path, body, ok, fail) {
        var xhr = new XMLHttpRequest();
        try {
            xhr.open(method, store.base + path);
        } catch (e) {
            if (fail) fail("" + e, 0);
            return null;
        }
        if (body !== null && body !== undefined)
            xhr.setRequestHeader("Content-Type", "application/json");
        xhr.onreadystatechange = function () {
            if (xhr.readyState !== 4)
                return;
            if (xhr.status >= 200 && xhr.status < 300) {
                var parsed = null;
                if (xhr.responseText && xhr.responseText.length > 0) {
                    try {
                        parsed = JSON.parse(xhr.responseText);
                    } catch (err) {
                        if (fail) fail("плохой JSON: " + err, xhr.status);
                        return;
                    }
                }
                if (ok) ok(parsed);
            } else if (fail) {
                fail(store.errorOf(xhr), xhr.status);
            }
        };
        try {
            xhr.send(body === null || body === undefined ? undefined : JSON.stringify(body));
        } catch (e2) {
            if (fail) fail("" + e2, 0);
        }
        return xhr;
    }

    // The error responses are `{"ok":false,"error":"…"}`; prefer that text over
    // a bare status code, it is the only place the adb failure is spelled out.
    function errorOf(xhr) {
        if (xhr.status === 0)
            return "демон не отвечает";
        if (xhr.responseText && xhr.responseText.length > 0) {
            try {
                var parsed = JSON.parse(xhr.responseText);
                if (parsed && parsed.error && ("" + parsed.error).length > 0)
                    return "" + parsed.error;
            } catch (e) {
                // fall through to the status code
            }
        }
        return "HTTP " + xhr.status;
    }

    function markOnline() {
        online = true;
        everAnswered = true;
        lastError = "";
    }
    function markOffline(msg) {
        online = false;
        lastError = msg;
    }

    // =====================================================================
    // loaders
    // =====================================================================
    // A poll answer can be in flight when the user flips a toggle, and it
    // still carries the pre-flip settings — adopting it would snap the control
    // back for a second. `settingsGuard` keeps the local mirror authoritative
    // until the POST answers (the POST clears the guard itself).
    property real settingsGuard: 0

    function adopt(payload) {
        markOnline();
        status = payload;
        // The daemon owns the settings; adopt them on every answer so the
        // page reflects what the bridge actually runs with.
        if (payload && payload.settings && new Date().getTime() >= settingsGuard) {
            settings = payload.settings;
            settingsLoaded = true;
        }
    }

    function refresh() {
        request("GET", "/api/status", null, function (r) {
            adopt(r);
        }, function (msg) {
            markOffline(msg);
            status = null;
        });
    }

    // =====================================================================
    // commands
    // =====================================================================
    // action: play | pause | play-pause | next | previous | stop.
    // The answer already carries the fresh `now`, so the UI updates from the
    // command itself — no extra GET, no half-second of stale artwork state.
    function cmd(action) {
        if (busyAction.length > 0)
            return;
        busyAction = action;
        request("POST", "/api/cmd", { action: action }, function (r) {
            markOnline();
            busyAction = "";
            if (r && r.now)
                status = patchedNow(r.now);
        }, function (msg, code) {
            busyAction = "";
            // 503 = daemon alive, container gone: stay "online" so the page
            // keeps showing the daemon-side facts and blame the container.
            if (code === 0)
                markOffline(msg);
            else
                lastError = msg;
            toast("Команда не прошла: " + msg);
            refresh();
        });
    }

    // `status` is replaced wholesale rather than mutated: assigning into a var
    // object does not notify bindings, so the pages would keep the old track.
    function patchedNow(fresh) {
        var next = {};
        if (status)
            for (var k in status) next[k] = status[k];
        next.now = fresh;
        return next;
    }

    // =====================================================================
    // settings
    // =====================================================================
    // POST /api/settings takes a partial document, so only the touched key
    // travels. The local mirror is updated first so the control does not lag
    // behind the finger by a round trip; the answer is a full status document.
    function saveSettings(patch) {
        var next = {};
        for (var k in settings) next[k] = settings[k];
        for (var p in patch) next[p] = patch[p];
        settings = next;
        settingsGuard = new Date().getTime() + 4000;

        request("POST", "/api/settings", patch, function (r) {
            settingsGuard = 0;
            adopt(r);
        }, function (msg, code) {
            settingsGuard = 0;
            if (code === 0)
                markOffline(msg);
            else
                lastError = msg;
            toast("Не удалось сохранить настройку");
            refresh();
        });
    }

    function setSetting(key, value) {
        var patch = {};
        patch[key] = value;
        saveSettings(patch);
    }

    // =====================================================================
    // formatting shared by the pages
    // =====================================================================
    function statusTitle(state) {
        if (state === "Playing") return "Играет";
        if (state === "Paused") return "На паузе";
        return "Остановлено";
    }

    function two(n) {
        return n < 10 ? "0" + n : "" + n;
    }

    function clock(ms) {
        var total = Math.floor((ms || 0) / 1000);
        if (total < 0) total = 0;
        var m = Math.floor(total / 60);
        var s = total - m * 60;
        if (m < 60) return m + ":" + two(s);
        return Math.floor(m / 60) + ":" + two(m % 60) + ":" + two(s);
    }

    function uptimeText(sec) {
        if (!sec || sec < 0) return "—";
        if (sec < 60) return sec + " с";
        if (sec < 3600) return Math.floor(sec / 60) + " мин";
        if (sec < 86400) return Math.floor(sec / 3600) + " ч " + Math.floor((sec % 3600) / 60) + " мин";
        return Math.floor(sec / 86400) + " дн " + Math.floor((sec % 86400) / 3600) + " ч";
    }

    function secondsText(ms) {
        var sec = (ms || 0) / 1000;
        return (sec >= 10 ? Math.round(sec) : sec.toFixed(1).replace(".0", "")) + " с";
    }

    // last_poll_ms is a wall-clock epoch in milliseconds; a relative age is
    // the only reading of it that means anything ("2 с назад" = alive).
    function agoText(ms) {
        if (!ms) return "";
        var sec = Math.round((new Date().getTime() - ms) / 1000);
        if (sec < 0) sec = 0;
        if (sec < 3) return "только что";
        if (sec < 60) return sec + " с назад";
        if (sec < 3600) return Math.floor(sec / 60) + " мин назад";
        if (sec < 86400) return Math.floor(sec / 3600) + " ч назад";
        return Math.floor(sec / 86400) + " дн назад";
    }

    // =====================================================================
    // lifecycle
    // =====================================================================
    // Polling is the only update path: the daemon has no event stream. One
    // second is fast enough for a track change to feel immediate and cheap
    // enough — the daemon answers from its own cache, it does not touch adb.
    function start() {
        refresh();
        poll.start();
    }

    // Called from Main on Qt.application.state changes. A backgrounded app
    // must not keep the daemon (and the container behind it) busy; coming back
    // refreshes at once so the first frame is never a stale track.
    function setActive(active) {
        if (active) {
            refresh();
            poll.start();
        } else {
            poll.stop();
        }
    }

    property Timer poll: Timer {
        interval: 1000
        repeat: true
        onTriggered: store.refresh()
    }
}
