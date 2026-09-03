import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

// Scrolling body shared by both pages: one padded column, consistent gutters.
Flickable {
    id: root

    property real gutter: Wm.l
    property real bottomInset: units.gu(3)
    property alias spacing: col.spacing

    default property alias body: col.data

    clip: true
    contentWidth: width
    contentHeight: col.height + gutter + bottomInset
    boundsBehavior: Flickable.DragOverBounds
    flickDeceleration: 3500

    Column {
        id: col
        x: root.gutter
        y: root.gutter
        width: root.width - 2 * root.gutter
        spacing: Wm.l
    }
}
