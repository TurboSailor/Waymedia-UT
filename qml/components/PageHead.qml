import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

// Page header, left aligned. The trailing button is anchored to the right edge
// and the title's right border is that button's left edge, so a long title
// elides instead of pushing the button off a narrow screen.
Item {
    id: root

    property string kicker: ""
    property string title: ""
    property string trailingGlyph: ""
    signal trailing()

    // A positioner (Column) does not size its children: without this the head
    // is 0 wide, the right-anchored button lands at a negative x and the title
    // column collapses to a negative width.
    width: parent ? parent.width : 0
    height: Math.max(col.height, units.gu(5))

    Column {
        id: col
        anchors.left: parent.left
        anchors.right: trailingBtn.visible ? trailingBtn.left : parent.right
        anchors.rightMargin: Wm.m
        anchors.top: parent.top
        spacing: units.dp(2)

        Label {
            visible: root.kicker.length > 0
            text: root.kicker.toUpperCase()
            color: Wm.textDim
            font.family: Wm.face
            font.pixelSize: Wm.micro
            font.weight: Font.DemiBold
            font.letterSpacing: units.dp(1.4)
        }

        Label {
            width: parent.width
            text: root.title
            color: Wm.text
            font.family: Wm.face
            font.pixelSize: Wm.headline
            font.weight: Font.Light
            elide: Text.ElideRight
        }
    }

    Item {
        id: trailingBtn
        visible: root.trailingGlyph.length > 0
        width: visible ? units.gu(4.5) : 0
        height: units.gu(4.5)
        anchors.right: parent.right
        anchors.top: parent.top

        Rectangle {
            anchors.fill: parent
            radius: width / 2
            color: Wm.card
        }
        Glyph {
            anchors.centerIn: parent
            name: root.trailingGlyph
            size: units.gu(2.5)
            color: Wm.text
        }
        MouseArea {
            anchors.fill: parent
            onClicked: root.trailing()
        }
    }
}
