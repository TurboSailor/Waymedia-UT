import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

// Empty states always say what to do next, never just "nothing here".
Column {
    id: root

    property string glyph: "music"
    property string title: ""
    property string hint: ""

    spacing: Wm.m

    Glyph {
        name: root.glyph
        size: units.gu(4)
        color: Wm.textDim
        weight: 1.5
    }

    Label {
        width: root.width
        text: root.title
        color: Wm.text
        font.family: Wm.face
        font.pixelSize: Wm.subtitle
        font.weight: Font.DemiBold
        wrapMode: Text.WordWrap
    }

    Label {
        width: root.width
        visible: root.hint.length > 0
        text: root.hint
        color: Wm.textDim
        font.family: Wm.face
        font.pixelSize: Wm.body
        lineHeight: 1.25
        wrapMode: Text.WordWrap
    }
}
