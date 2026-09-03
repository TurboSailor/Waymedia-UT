import QtQuick 2.4
import Ubuntu.Components 1.3
import "theme"
import "store"
import "components"
import "pages"

// Тонкий клиент демона waymediad: сам он живёт systemd-юнитом и держит мост
// без UI, приложение только показывает состояние и правит настройки.
MainView {
    id: app

    applicationName: "waymedia.turbosailor"
    objectName: "waymediaMain"
    anchorToKeyboard: true
    backgroundColor: Wm.bg

    // Реальная геометрия устройства: GRID_UNIT_PX=24, 1080x2400 -> 45x100 gu.
    width: units.gu(45)
    height: units.gu(100)

    // Lomiri ships a dark and a light Suru theme; follow whichever is active
    // instead of guessing. `theme` is a UITK StyledItem property — probing it
    // defensively keeps the app silent if it is ever absent.
    readonly property bool systemDark: {
        try {
            return ("" + theme.name).toLowerCase().indexOf("dark") >= 0;
        } catch (e) {
            return true;
        }
    }
    onSystemDarkChanged: Wm.systemDark = systemDark

    readonly property int toastSeq: Store.toastSeq
    onToastSeqChanged: toast.show(Store.toastText)

    // Одна секунда опроса имеет смысл только пока окно на экране: свёрнутое
    // приложение не должно гонять демон (и контейнер за ним). Обработчик, а не
    // Connections{enabled:} — последнего в QtQuick 2.4 нет.
    readonly property int appState: Qt.application.state
    onAppStateChanged: Store.setActive(appState === Qt.ApplicationActive)

    Component.onCompleted: {
        Wm.systemDark = systemDark;
        Store.start();
    }

    readonly property var tabs: [
        { key: "now",      label: "Сейчас",    glyph: "music" },
        { key: "settings", label: "Настройки", glyph: "sliders" }
    ]

    property int tab: 0
    // Страница остаётся живой после первого визита: позиция прокрутки и
    // состояние списков переживают переключение вкладок.
    property var visited: [true, false]

    function showTab(i) {
        if (i === tab) return;
        var v = visited.slice();
        v[i] = true;
        visited = v;
        tab = i;
    }

    Rectangle {
        anchors.fill: parent
        color: Wm.bg
    }

    Item {
        id: pageHost
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: nav.top

        Repeater {
            model: app.tabs.length

            delegate: Loader {
                anchors.fill: parent
                active: app.visited[index]
                visible: opacity > 0
                opacity: app.tab === index ? 1 : 0

                Behavior on opacity { NumberAnimation { duration: Wm.med; easing.type: Easing.OutQuart } }

                sourceComponent: index === 0 ? nowComponent : settingsComponent
            }
        }
    }

    Component {
        id: nowComponent
        NowPlayingPage {}
    }

    Component {
        id: settingsComponent
        SettingsPage {}
    }

    NavBar {
        id: nav
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        tabs: app.tabs
        current: app.tab
        onPicked: app.showTab(index)
    }

    Toast {
        id: toast
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: nav.top
        anchors.bottomMargin: Wm.l
    }
}
