import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

Item {
    id: root

    property bool checked: false
    signal toggled(bool value)

    width: units.gu(5.5)
    height: units.gu(3)

    Rectangle {
        anchors.fill: parent
        radius: height / 2
        color: root.checked ? Wm.accent : Wm.cardAlt
        Behavior on color { ColorAnimation { duration: Wm.med } }
    }

    Rectangle {
        width: parent.height - units.dp(6)
        height: width
        radius: width / 2
        y: units.dp(3)
        x: root.checked ? parent.width - width - units.dp(3) : units.dp(3)
        color: root.checked ? Wm.accentInk : Wm.textDim
        Behavior on x { NumberAnimation { duration: Wm.med; easing.type: Easing.OutQuart } }
        Behavior on color { ColorAnimation { duration: Wm.med } }
    }

    MouseArea {
        anchors.fill: parent
        anchors.margins: -Wm.s
        onClicked: root.toggled(!root.checked)
    }
}
