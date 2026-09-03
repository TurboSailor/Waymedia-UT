import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"

// Marks on a 24x24 grid, rasterised through the SVG image plugin.
//
// QtQuick.Shapes draws nothing inside Ubuntu.Components' MainView on this
// platform (verified on device: the identical ShapePath renders standalone and
// stays blank under MainView, with no error in the journal), so the path data
// goes to an Image with an SVG data URI instead.
//
// Transport marks read wrong as outlines, so `filled` paints the path solid;
// the stroke stays on top of it to round the corners of the triangles.
Item {
    id: root

    property string name: ""
    property real size: units.gu(2.5)
    property color color: Wm.text
    property real weight: 1.9
    property bool filled: false

    width: size
    height: size

    readonly property string path: {
        switch (name) {
        // ---- transport (filled) -----------------------------------------
        case "play":
            return "M8.4 5.4L18.6 12L8.4 18.6z";
        case "pause":
            return "M8.4 5.6h2.6v12.8H8.4z M13 5.6h2.6v12.8H13z";
        case "stop":
            return "M6.6 6.6h10.8v10.8H6.6z";
        case "previous":
            return "M18.2 5.6L9.4 12l8.8 6.4z M6.2 5.6h2.2v12.8H6.2z";
        case "next":
            return "M5.8 5.6L14.6 12 5.8 18.4z M15.6 5.6h2.2v12.8h-2.2z";
        // ---- outlines ----------------------------------------------------
        case "music":
            return "M9.4 17.8V5.4l9.2-1.8v12.2" +
                   "M9.4 17.8a2.6 2.6 0 1 1-5.2 0 2.6 2.6 0 0 1 5.2 0z" +
                   "M18.6 15.8a2.6 2.6 0 1 1-5.2 0 2.6 2.6 0 0 1 5.2 0z";
        case "link":
            return "M9.6 14.4l4.8-4.8" +
                   "M8.6 12.2L6.4 14.4a3.2 3.2 0 0 0 4.6 4.6l2.2-2.2" +
                   "M15.4 11.8l2.2-2.2a3.2 3.2 0 0 0-4.6-4.6l-2.2 2.2";
        case "android":
            return "M6.2 10.4h11.6v7a2 2 0 0 1-2 2h-7.6a2 2 0 0 1-2-2zM9.2 10.4a2.8 2.8 0 0 1 5.6 0M8.6 6.6L7.2 4.4M15.4 6.6L16.8 4.4";
        case "sliders":
            return "M4 7.2h9M17.4 7.2H20M4 16.8h4.4M12.8 16.8H20M15.2 4.4v5.6M10.6 14v5.6";
        case "sync":
            return "M20 12a8 8 0 1 1-2.4-5.7M20.2 3.4v5.2h-5.2";
        case "close":
            return "M6 6l12 12M18 6L6 18";
        }
        return "";
    }

    // Qt stringifies a color as #aarrggbb, which SVG does not understand, so
    // the channels are written out and the alpha travels as *-opacity.
    function rgbOf(c) {
        return "rgb(" + Math.round(c.r * 255) + "," +
                        Math.round(c.g * 255) + "," +
                        Math.round(c.b * 255) + ")";
    }

    readonly property string svgSource:
        "data:image/svg+xml;utf8," +
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">' +
        '<path d="' + path + '"' +
        ' fill="' + (filled ? rgbOf(color) : "none") + '"' +
        (filled ? ' fill-opacity="' + color.a.toFixed(3) + '"' : '') +
        ' stroke="' + rgbOf(color) + '" stroke-opacity="' + color.a.toFixed(3) + '"' +
        ' stroke-width="' + (filled ? 1.2 : weight) + '"' +
        ' stroke-linecap="round" stroke-linejoin="round"/></svg>'

    Image {
        anchors.fill: parent
        visible: root.path.length > 0
        smooth: true
        // Rasterise at device pixels so the stroke stays crisp.
        sourceSize.width: Math.max(1, Math.round(root.size))
        sourceSize.height: Math.max(1, Math.round(root.size))
        source: root.path.length > 0 ? root.svgSource : ""
    }
}
