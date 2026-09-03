pragma Singleton
import QtQuick 2.4
import Ubuntu.Components 1.3

// Design tokens. One accent (the Android green the bridge comes from), dark and
// light surfaces that follow the Lomiri theme, and a single spacing scale —
// every size in the app comes from here, so nothing is expressed in raw pixels.
QtObject {
    id: wm

    // Main.qml feeds the active Suru theme in; "dark" is the honest default
    // because the shell ships dark by default on the phone.
    property bool systemDark: true
    readonly property bool dark: systemDark

    // ---- surfaces ------------------------------------------------------
    readonly property color bg:       dark ? "#0E1420" : "#F4F5F7"
    readonly property color card:     dark ? "#151C2A" : "#FFFFFF"
    readonly property color cardAlt:  dark ? "#1E2637" : "#E4E7EC"
    readonly property color hairline: dark ? Qt.rgba(1, 1, 1, 0.07) : Qt.rgba(0, 0, 0, 0.08)

    // ---- ink -----------------------------------------------------------
    readonly property color text:     dark ? "#ECEFF4" : "#16181D"
    readonly property color textDim:  dark ? "#8B93A3" : "#6B6F78"
    // Ubuntu orange carries white ink, exactly like the shell's own positive
    // buttons — the icon does the same (white play mark inside an orange ring).
    // NB: not named `onAccent`. QML parses a member whose name starts with
    // "on" + capital as a signal handler, so such a "property" is silently
    // never assigned and reads back as black.
    readonly property color accentInk: "#FFFFFF"

    // ---- signal colours -------------------------------------------------
    // Brand accent is the app icon's orange, so `ok` carries the "this works"
    // meaning instead: orange on a status line would read as a warning.
    readonly property color accent: dark ? "#E95420" : "#D14310"
    readonly property color ok:     dark ? "#3DDC84" : "#1BA05C"
    readonly property color warn:   dark ? "#F5C542" : "#A8760A"
    readonly property color bad:    dark ? "#FF6B6B" : "#D93A3A"

    // ---- rhythm ---------------------------------------------------------
    readonly property real s:   units.gu(1)
    readonly property real m:   units.gu(1.5)
    readonly property real l:   units.gu(2)
    readonly property real xl:  units.gu(3)

    readonly property real radiusCard: units.gu(2)
    readonly property real radiusPill: units.gu(1.25)

    // ---- type -----------------------------------------------------------
    readonly property string face: "Ubuntu"
    readonly property int micro:    units.dp(10)
    readonly property int caption:  units.dp(12)
    readonly property int body:     units.dp(14)
    readonly property int subtitle: units.dp(16)
    readonly property int title:    units.dp(21)
    readonly property int headline: units.dp(26)

    // ---- motion ---------------------------------------------------------
    readonly property int fast: 140
    readonly property int med:  240

    function alpha(c, a) {
        return Qt.rgba(c.r, c.g, c.b, a);
    }
}
