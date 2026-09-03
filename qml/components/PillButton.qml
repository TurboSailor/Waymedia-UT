import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

Rectangle {
    id: root

    property string text: ""
    property string glyph: ""
    // Транспортные знаки залиты, чтобы «стоп» на кнопке читался так же, как
    // на круглых клавишах, а не тонким контуром.
    property bool glyphFilled: false
    // "primary" | "ghost" | "danger"
    property string kind: "ghost"
    property bool busy: false
    property bool enabledLook: true
    signal clicked()

    readonly property color base: kind === "primary" ? Wm.accent
                                : kind === "danger" ? Wm.alpha(Wm.bad, 0.16)
                                : Wm.cardAlt
    readonly property color ink: kind === "primary" ? Wm.accentInk
                               : kind === "danger" ? Wm.bad
                               : Wm.text

    implicitHeight: units.gu(5)
    implicitWidth: row.width + Wm.xl
    radius: Wm.radiusPill
    color: base
    opacity: enabledLook ? 1 : 0.4

    Row {
        id: row
        anchors.centerIn: parent
        spacing: Wm.s

        Glyph {
            visible: root.glyph.length > 0 && !root.busy
            name: root.glyph
            size: units.gu(2.25)
            filled: root.glyphFilled
            color: root.ink
            anchors.verticalCenter: parent.verticalCenter
        }

        Glyph {
            visible: root.busy
            name: "sync"
            size: units.gu(2.25)
            color: root.ink
            anchors.verticalCenter: parent.verticalCenter
            RotationAnimator on rotation {
                running: root.busy
                loops: Animation.Infinite
                from: 0; to: 360; duration: 1100
            }
        }

        Label {
            text: root.text
            color: root.ink
            font.family: Wm.face
            font.pixelSize: Wm.body
            font.weight: Font.DemiBold
            anchors.verticalCenter: parent.verticalCenter
        }
    }

    Rectangle {
        anchors.fill: parent
        radius: parent.radius
        color: Wm.text
        opacity: tap.pressed ? 0.10 : 0
        Behavior on opacity { NumberAnimation { duration: Wm.fast } }
    }

    MouseArea {
        id: tap
        anchors.fill: parent
        enabled: root.enabledLook
        onClicked: root.clicked()
    }
}
