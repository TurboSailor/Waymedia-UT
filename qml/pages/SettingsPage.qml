import QtQuick 2.4
import Ubuntu.Components 1.3
import "../theme"
import "../store"
import "../components"

// Настройки демона. Всё пишется сразу по факту переключения (POST /api/settings
// принимает частичный документ), кнопки «Сохранить» нет намеренно.
Item {
    id: page

    readonly property var cfg: Store.settings
    readonly property var br: Store.bridge
    readonly property var wd: Store.waydroid
    readonly property string bridgeError: br && br.error ? "" + br.error : ""

    Screen {
        anchors.fill: parent

        PageHead {
            kicker: "демон waymediad"
            title: "Настройки"
            trailingGlyph: "sync"
            onTrailing: Store.refresh()
        }

        // ---- мост -----------------------------------------------------------
        Card {
            width: parent.width

            SettingRow {
                title: "Мост включён"
                subtitle: cfg.enabled
                          ? "Плеер Waydroid виден системе как MPRIS-плеер"
                          : "Мост выключен, система плеера не видит"
                glyph: "link"
                divider: false
                Toggle {
                    checked: !!cfg.enabled
                    onToggled: Store.setSetting("enabled", value)
                }
            }

            Label {
                width: parent.width
                text: "Пока мост включён, трек и кнопки Waydroid появляются в индикаторе звука — " +
                      "в том числе на экране блокировки, где можно переключать треки не разблокируя телефон. " +
                      "Выключение сразу убирает плеер из индикатора."
                color: Wm.textDim
                font.family: Wm.face
                font.pixelSize: Wm.caption
                lineHeight: 1.25
                wrapMode: Text.WordWrap
            }
        }

        // ---- опрос ----------------------------------------------------------
        Card {
            width: parent.width

            Label {
                width: parent.width
                text: "Опрос контейнера"
                color: Wm.text
                font.family: Wm.face
                font.pixelSize: Wm.subtitle
                font.weight: Font.DemiBold
            }

            Label {
                width: parent.width
                text: "Каждый опрос — это dumpsys media_session в Waydroid. Чаще опрос — быстрее " +
                      "реакция на смену трека и больше расход батареи."
                color: Wm.textDim
                font.family: Wm.face
                font.pixelSize: Wm.caption
                lineHeight: 1.25
                wrapMode: Text.WordWrap
            }

            SectionTitle {
                width: parent.width
                text: "когда играет"
            }

            Segmented {
                width: parent.width
                options: [
                    { key: "1000", label: "1 с" },
                    { key: "1500", label: "1.5 с" },
                    { key: "3000", label: "3 с" }
                ]
                current: "" + (cfg.poll_ms || 1500)
                onPicked: Store.setSetting("poll_ms", parseInt(key, 10))
            }

            SectionTitle {
                width: parent.width
                text: "когда тишина"
            }

            Segmented {
                width: parent.width
                options: [
                    { key: "3000",  label: "3 с" },
                    { key: "5000",  label: "5 с" },
                    { key: "10000", label: "10 с" }
                ]
                current: "" + (cfg.idle_poll_ms || 5000)
                onPicked: Store.setSetting("idle_poll_ms", parseInt(key, 10))
            }

            Label {
                width: parent.width
                text: "Без активной медиа-сессии демон опрашивает контейнер реже: " +
                      Store.secondsText(cfg.idle_poll_ms || 5000) +
                      " вместо " + Store.secondsText(cfg.poll_ms || 1500) +
                      ". Это основной вклад в батарею — держать редкий тихий опрос выгодно."
                color: Wm.textDim
                font.family: Wm.face
                font.pixelSize: Wm.caption
                lineHeight: 1.25
                wrapMode: Text.WordWrap
            }
        }

        // ---- диагностика ----------------------------------------------------
        SectionTitle {
            width: parent.width
            text: "Диагностика"
            action: "обновить"
            onActionTriggered: Store.refresh()
        }

        Card {
            width: parent.width
            padding: Wm.m
            spacing: 0

            FactRow {
                label: "Имя на шине"
                value: br && br.bus_name ? br.bus_name : ""
                tone: br && br.mpris_exported ? Wm.text : Wm.textDim
            }

            FactRow {
                label: "Desktop id"
                value: {
                    var id = br && br.desktop_id ? "" + br.desktop_id : "";
                    if (id.length === 0) return "";
                    // Индикатор звука находит плеер только если .desktop
                    // резолвится — это единственное условие показа.
                    return id + (br.desktop_registered ? " · найден" : " · не найден");
                }
                tone: br && br.desktop_registered ? Wm.text : Wm.warn
            }

            FactRow {
                label: "Транспорт"
                value: wd && wd.transport ? wd.transport : ""
            }

            FactRow {
                label: "Последний опрос"
                value: wd ? Store.agoText(wd.last_poll_ms) : ""
            }

            FactRow {
                label: "Демон"
                value: Store.online
                       ? "waymediad " + (Store.version.length > 0 ? Store.version : "?") +
                         " · " + Store.uptimeText(Store.uptimeSec)
                       : "не отвечает"
                tone: Store.online ? Wm.text : Wm.bad
                divider: page.bridgeError.length > 0
            }

            FactRow {
                label: "Ошибка моста"
                visible: page.bridgeError.length > 0
                value: page.bridgeError
                tone: Wm.bad
                divider: false
            }
        }

        Label {
            width: parent.width
            text: "Демон работает systemd-юнитом и не зависит от этого окна: " +
                  "закрытое приложение мост не выключает."
            color: Wm.textDim
            font.family: Wm.face
            font.pixelSize: Wm.micro
            wrapMode: Text.WordWrap
        }
    }
}
