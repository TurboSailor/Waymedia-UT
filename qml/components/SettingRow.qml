import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

// One settings line. The default slot takes whatever control the setting needs
// (a Toggle, a value label) and keeps it right aligned.
Item {
    id: root

    property string title: ""
    property string subtitle: ""
    property string glyph: ""
    property bool divider: true

    default property alias controlData: control.data

    width: parent ? parent.width : 0
    implicitHeight: Math.max(texts.height, control.height) + 2 * Wm.m

    Glyph {
        id: icon
        visible: root.glyph.length > 0
        name: root.glyph
        size: units.gu(2.25)
        color: Wm.textDim
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
    }

    Column {
        id: texts
        anchors.left: icon.visible ? icon.right : parent.left
        anchors.leftMargin: icon.visible ? Wm.m : 0
        anchors.right: control.left
        anchors.rightMargin: Wm.m
        anchors.verticalCenter: parent.verticalCenter
        spacing: units.dp(2)

        Label {
            width: parent.width
            text: root.title
            color: Wm.text
            font.family: Wm.face
            font.pixelSize: Wm.body
            elide: Text.ElideRight
        }
        Label {
            width: parent.width
            visible: root.subtitle.length > 0
            text: root.subtitle
            color: Wm.textDim
            font.family: Wm.face
            font.pixelSize: Wm.caption
            wrapMode: Text.WordWrap
        }
    }

    Item {
        id: control
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        width: childrenRect.width
        height: childrenRect.height
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
