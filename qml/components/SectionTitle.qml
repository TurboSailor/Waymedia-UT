import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

Item {
    id: root

    property string text: ""
    property string action: ""
    signal actionTriggered()

    height: Math.max(label.height, link.height, units.gu(2))

    Label {
        id: label
        anchors.left: parent.left
        anchors.right: link.visible ? link.left : parent.right
        anchors.rightMargin: Wm.s
        anchors.verticalCenter: parent.verticalCenter
        text: root.text.toUpperCase()
        color: Wm.textDim
        font.family: Wm.face
        font.pixelSize: Wm.micro
        font.weight: Font.DemiBold
        font.letterSpacing: units.dp(1.4)
        elide: Text.ElideRight
    }

    Label {
        id: link
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        visible: root.action.length > 0
        text: root.action
        color: Wm.accent
        font.family: Wm.face
        font.pixelSize: Wm.caption

        MouseArea {
            anchors.fill: parent
            anchors.margins: -Wm.s
            onClicked: root.actionTriggered()
        }
    }
}
