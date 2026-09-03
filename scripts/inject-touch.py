#!/usr/bin/env python3
"""Инжектор тач-свайпа через evdev — нужен только для проверки на устройстве.

Из adb нельзя ни запустить приложение (нет trust-session), ни открыть шторку
индикаторов, поэтому единственный способ увидеть медиа-контролы на экране
блокировки своими глазами — послать свайп прямо в тачскрин. phablet входит в
группу android_input, так что /dev/input/eventN доступен на запись без root.

Протокол B: слот + tracking id, координаты в единицах устройства.
"""
import fcntl
import struct
import sys
import time

EV_SYN, EV_KEY, EV_ABS = 0x00, 0x01, 0x03
SYN_REPORT = 0
BTN_TOUCH = 0x14A
ABS_MT_SLOT = 0x2F
ABS_MT_POSITION_X, ABS_MT_POSITION_Y = 0x35, 0x36
ABS_MT_TRACKING_ID = 0x39
ABS_MT_PRESSURE = 0x3A

FORMAT = "llHHi"  # timeval + type + code + value


def emit(f, etype, code, value):
    f.write(struct.pack(FORMAT, 0, 0, etype, code, value))


def sync(f):
    emit(f, EV_SYN, SYN_REPORT, 0)
    f.flush()


def swipe(dev, x0, y0, x1, y1, steps=24, delay=0.012):
    with open(dev, "wb", buffering=0) as f:
        emit(f, EV_ABS, ABS_MT_SLOT, 0)
        emit(f, EV_ABS, ABS_MT_TRACKING_ID, 77)
        emit(f, EV_ABS, ABS_MT_POSITION_X, x0)
        emit(f, EV_ABS, ABS_MT_POSITION_Y, y0)
        emit(f, EV_ABS, ABS_MT_PRESSURE, 60)
        emit(f, EV_KEY, BTN_TOUCH, 1)
        sync(f)
        for i in range(1, steps + 1):
            emit(f, EV_ABS, ABS_MT_SLOT, 0)
            emit(f, EV_ABS, ABS_MT_POSITION_X, int(x0 + (x1 - x0) * i / steps))
            emit(f, EV_ABS, ABS_MT_POSITION_Y, int(y0 + (y1 - y0) * i / steps))
            emit(f, EV_ABS, ABS_MT_PRESSURE, 60)
            sync(f)
            time.sleep(delay)
        emit(f, EV_ABS, ABS_MT_SLOT, 0)
        emit(f, EV_ABS, ABS_MT_TRACKING_ID, -1)
        emit(f, EV_KEY, BTN_TOUCH, 0)
        sync(f)


def tap(dev, x, y, hold=0.08):
    with open(dev, "wb", buffering=0) as f:
        emit(f, EV_ABS, ABS_MT_SLOT, 0)
        emit(f, EV_ABS, ABS_MT_TRACKING_ID, 78)
        emit(f, EV_ABS, ABS_MT_POSITION_X, x)
        emit(f, EV_ABS, ABS_MT_POSITION_Y, y)
        emit(f, EV_ABS, ABS_MT_PRESSURE, 60)
        emit(f, EV_KEY, BTN_TOUCH, 1)
        sync(f)
        time.sleep(hold)
        emit(f, EV_ABS, ABS_MT_SLOT, 0)
        emit(f, EV_ABS, ABS_MT_TRACKING_ID, -1)
        emit(f, EV_KEY, BTN_TOUCH, 0)
        sync(f)


if __name__ == "__main__":
    mode = sys.argv[1]
    dev = sys.argv[2]
    coords = [int(v) for v in sys.argv[3:]]
    if mode == "swipe":
        swipe(dev, *coords)
    elif mode == "tap":
        tap(dev, *coords)
    else:
        raise SystemExit("usage: swipe.py {swipe|tap} /dev/input/eventN coords...")
    print("ok")
