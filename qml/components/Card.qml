import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

// Base surface: a padded vertical stack. Every panel is this rectangle;
// nothing nests cards inside cards, depth is expressed with the alt tone.
Rectangle {
    id: root

    property real padding: Wm.l
    property real spacing: Wm.m
    property bool alt: false

    default property alias contentData: content.data

    color: alt ? Wm.cardAlt : Wm.card
    radius: Wm.radiusCard
    implicitHeight: content.height + 2 * padding

    Column {
        id: content
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: root.padding
        spacing: root.spacing
    }
}
