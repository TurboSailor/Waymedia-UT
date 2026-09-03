import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"
import "../store"
import "../components"

// Что играет в Waydroid прямо сейчас и куда это уехало: MPRIS на сессионной
// шине, индикатор звука, экран блокировки. Кнопки транспорта здесь — те же
// команды, что и на локскрине, только с обратной связью.
Item {
    id: page

    readonly property var now: Store.now
    readonly property var wd: Store.waydroid
    readonly property var br: Store.bridge

    readonly property bool live: !!(now && now.has_session)
    readonly property bool playing: !!(now && now.status === "Playing")

    // Имя приложения приходит не всегда (Android отдаёт label лениво), тогда
    // честнее показать пакет, чем пустую строку.
    readonly property string appTitle: {
        if (!now) return "";
        if (now.app_name && now.app_name.length > 0) return now.app_name;
        if (now["package"] && now["package"].length > 0) return now["package"];
        return "";
    }

    readonly property string wdError: wd && wd.error ? "" + wd.error : ""
    readonly property string brError: br && br.error ? "" + br.error : ""

    function send(action) {
        Store.cmd(action);
    }

    Screen {
        anchors.fill: parent

        PageHead {
            kicker: "waydroid → mpris"
            title: "WayMedia"
            trailingGlyph: "sync"
            onTrailing: Store.refresh()
        }

        // ---- сейчас играет -------------------------------------------------
        Card {
            width: parent.width
            spacing: Wm.l

            // Шапка карточки: приложение-источник.
            Item {
                width: parent.width
                height: units.gu(3)
                visible: page.live

                Glyph {
                    id: srcIcon
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    name: "android"
                    size: units.gu(2.25)
                    color: Wm.ok
                }

                Label {
                    anchors.left: srcIcon.right
                    anchors.leftMargin: Wm.s
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: page.appTitle
                    color: Wm.textDim
                    font.family: Wm.face
                    font.pixelSize: Wm.caption
                    font.weight: Font.DemiBold
                    font.letterSpacing: units.dp(0.8)
                    elide: Text.ElideRight
                }
            }

            Column {
                width: parent.width
                visible: page.live
                spacing: units.dp(2)

                Label {
                    width: parent.width
                    text: now && now.title && now.title.length > 0 ? now.title : "(без названия)"
                    color: Wm.text
                    font.family: Wm.face
                    font.pixelSize: Wm.title
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }

                Label {
                    width: parent.width
                    visible: !!(now && now.artist && now.artist.length > 0)
                    text: now ? now.artist : ""
                    color: Wm.textDim
                    font.family: Wm.face
                    font.pixelSize: Wm.body
                    elide: Text.ElideRight
                }

                Label {
                    width: parent.width
                    visible: !!(now && now.album && now.album.length > 0)
                    text: now ? now.album : ""
                    color: Wm.textDim
                    font.family: Wm.face
                    font.pixelSize: Wm.caption
                    elide: Text.ElideRight
                }
            }

            // Статус и позиция одной строкой: точка цветом состояния слева,
            // время справа — прогресс-бара нет намеренно, позиция приходит
            // раз в секунду и врать плавностью незачем.
            Item {
                width: parent.width
                height: units.gu(2.5)
                visible: page.live

                Rectangle {
                    id: dot
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: units.gu(1)
                    height: width
                    radius: width / 2
                    color: page.playing ? Wm.ok
                         : (now && now.status === "Paused") ? Wm.warn : Wm.textDim
                }

                Label {
                    anchors.left: dot.right
                    anchors.leftMargin: Wm.s
                    anchors.right: posLabel.left
                    anchors.rightMargin: Wm.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: Store.statusTitle(now ? now.status : "Stopped")
                    color: Wm.text
                    font.family: Wm.face
                    font.pixelSize: Wm.caption
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }

                Label {
                    id: posLabel
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    visible: !!(now && now.position_ms > 0)
                    text: Store.clock(now ? now.position_ms : 0)
                    color: Wm.textDim
                    font.family: Wm.face
                    font.pixelSize: Wm.caption
                }
            }

            // ---- нечего играть ---------------------------------------------
            EmptyState {
                width: parent.width
                visible: !page.live
                glyph: Store.online ? "music" : "close"
                title: Store.online
                       ? "В Waydroid ничего не играет"
                       : "Демон не отвечает"
                hint: Store.online
                      ? "Откройте плеер в Waydroid и нажмите play — трек появится здесь, в индикаторе звука и на экране блокировки."
                      : "Проверьте: systemctl --user status waymediad"
            }

            // ---- транспорт -------------------------------------------------
            Item {
                width: parent.width
                height: playKey.height

                Row {
                    anchors.centerIn: parent
                    spacing: Wm.l

                    TransportButton {
                        anchors.verticalCenter: parent.verticalCenter
                        glyph: "previous"
                        diameter: units.gu(6)
                        active: page.live && !!now.can_previous
                        busy: Store.busyAction === "previous"
                        onTriggered: page.send("previous")
                    }

                    TransportButton {
                        id: playKey
                        glyph: page.playing ? "pause" : "play"
                        diameter: units.gu(8.5)
                        primary: true
                        // Пауза/пуск — единственная кнопка, которая нужна и в
                        // Stopped: can_play разрешает старт после остановки.
                        active: page.live && (page.playing ? !!now.can_pause : !!now.can_play)
                        busy: Store.busyAction === "play-pause"
                        onTriggered: page.send("play-pause")
                    }

                    TransportButton {
                        anchors.verticalCenter: parent.verticalCenter
                        glyph: "next"
                        diameter: units.gu(6)
                        active: page.live && !!now.can_next
                        busy: Store.busyAction === "next"
                        onTriggered: page.send("next")
                    }
                }
            }

            Item {
                width: parent.width
                height: stopKey.height
                visible: page.live && !!now.can_stop

                PillButton {
                    id: stopKey
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: "Остановить"
                    glyph: "stop"
                    glyphFilled: true
                    kind: "ghost"
                    busy: Store.busyAction === "stop"
                    enabledLook: page.live && !!now.can_stop
                    onClicked: page.send("stop")
                }
            }
        }

        // ---- состояние моста -----------------------------------------------
        Card {
            width: parent.width
            padding: Wm.m
            spacing: 0

            FactRow {
                label: "Waydroid"
                // Пока демон молчит, у нас нет данных о контейнере — это не то
                // же самое, что «контейнер лежит», и врать об этом нельзя.
                value: !wd ? "нет данных"
                     : (wd.session && wd.session.length > 0 ? wd.session : "неизвестно") +
                       (wd.reachable ? " · на связи" : " · нет связи")
                tone: !wd ? Wm.textDim : (wd.reachable ? Wm.ok : Wm.warn)
            }

            FactRow {
                label: "Мост MPRIS"
                value: !br ? "нет данных"
                     : !br.enabled ? "мост выключен"
                     : br.mpris_exported ? "экспортирован" : "не экспортирован"
                tone: !br ? Wm.textDim
                    : (br.enabled && br.mpris_exported ? Wm.ok : Wm.warn)
            }

            FactRow {
                label: "В индикаторе звука"
                // Индикатор берёт плеер с шины сам, но показывает его только
                // если резолвится .desktop из свойства DesktopEntry.
                value: !br ? "нет данных"
                     : !br.mpris_exported ? "нет"
                     : br.desktop_registered ? "да · и на экране блокировки"
                     : "да · .desktop не найден"
                tone: br && br.mpris_exported && br.desktop_registered ? Wm.ok : Wm.textDim
                divider: page.wdError.length > 0 || page.brError.length > 0
            }

            FactRow {
                label: "Ошибка контейнера"
                visible: page.wdError.length > 0
                value: page.wdError
                tone: Wm.bad
                divider: page.brError.length > 0
            }

            FactRow {
                label: "Ошибка моста"
                visible: page.brError.length > 0
                value: page.brError
                tone: Wm.bad
                divider: false
            }
        }

        Label {
            width: parent.width
            text: (Store.version.length > 0 ? "waymediad " + Store.version : "waymediad") +
                  (wd && wd.transport && wd.transport.length > 0 ? "  ·  " + wd.transport : "")
            color: Wm.textDim
            font.family: Wm.face
            font.pixelSize: Wm.micro
            horizontalAlignment: Text.AlignHCenter
            elide: Text.ElideRight
        }
    }
}
