import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

// One round transport key. Circles rather than pills: the triad reads as a
// single instrument, and the middle key is deliberately bigger because it is
// the one hit without looking.
Item {
    id: root

    property string glyph: "play"
    property real diameter: units.gu(6)
    property bool primary: false
    // Mirrors the daemon's can_* flags; a key the session cannot serve stays
    // visible (the layout must not jump) but does not answer.
    property bool active: true
    property bool busy: false

    signal triggered()

    width: diameter
    height: diameter
    opacity: active ? (busy ? 0.7 : 1) : 0.3
    Behavior on opacity { NumberAnimation { duration: Wm.fast } }

    Rectangle {
        anchors.fill: parent
        radius: width / 2
        color: root.primary ? Wm.accent : Wm.cardAlt
    }

    Glyph {
        anchors.centerIn: parent
        name: root.glyph
        size: root.diameter * 0.42
        filled: true
        weight: 1.2
        color: root.primary ? Wm.accentInk : Wm.text
    }

    Rectangle {
        anchors.fill: parent
        radius: width / 2
        color: Wm.text
        opacity: tap.pressed ? 0.12 : 0
        Behavior on opacity { NumberAnimation { duration: Wm.fast } }
    }

    MouseArea {
        id: tap
        anchors.fill: parent
        enabled: root.active
        onClicked: root.triggered()
    }
}
