import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

// Read-only fact: caption above, value below. The value gets the full width
// because the interesting ones here are long (a bus name, an adb transport,
// an adb error) — a right-aligned value column would elide them to nothing.
Item {
    id: root

    property string label: ""
    property string value: ""
    property color tone: Wm.text
    property bool divider: true

    width: parent ? parent.width : 0
    implicitHeight: col.height + 2 * Wm.s

    Column {
        id: col
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: units.dp(1)

        Label {
            text: root.label.toUpperCase()
            color: Wm.textDim
            font.family: Wm.face
            font.pixelSize: Wm.micro
            font.weight: Font.DemiBold
            font.letterSpacing: units.dp(1)
        }

        Label {
            width: parent.width
            text: root.value.length > 0 ? root.value : "—"
            color: root.tone
            font.family: Wm.face
            font.pixelSize: Wm.caption
            elide: Text.ElideRight
        }
    }

    Rectangle {
        visible: root.divider
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: units.dp(1)
        color: Wm.hairline
    }
}
