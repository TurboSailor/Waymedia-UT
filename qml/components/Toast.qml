import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

Rectangle {
    id: root

    property string message: ""

    function show(text) {
        message = text;
        life.restart();
        opacity = 1;
    }

    width: parent ? Math.min(parent.width - 2 * Wm.l, units.gu(40)) : 0
    height: label.implicitHeight + 2 * Wm.m
    radius: Wm.radiusPill
    color: Wm.cardAlt
    opacity: 0
    visible: opacity > 0

    Behavior on opacity { NumberAnimation { duration: Wm.med } }

    Label {
        id: label
        anchors.centerIn: parent
        width: root.width - 2 * Wm.l
        text: root.message
        color: Wm.text
        font.family: Wm.face
        font.pixelSize: Wm.body
        horizontalAlignment: Text.AlignHCenter
        wrapMode: Text.WordWrap
    }

    Timer {
        id: life
        interval: 2600
        onTriggered: root.opacity = 0
    }
}
