#!/usr/bin/env python3
"""Generate click/waymedia.png deterministically, without any image library.

The build hosts (macOS laptop, phone) have neither PIL nor rsvg-convert, and a
binary blob in git that nobody can regenerate is worse than a 100-line writer.
So the icon is rasterised here by hand: signed-distance shapes evaluated on a
supersampled grid, composited in premultiplied alpha, then serialised as a
single-IDAT RGBA8 PNG (zlib + hand-rolled chunk framing).

Usage: scripts/gen-icon.py [OUT_PNG]   (default: click/waymedia.png)
"""

import math
import os
import struct
import sys
import zlib

SIZE = 256           # click icons are consumed at 256x256 by Lomiri's launcher
SS = 4               # 4x4 samples per pixel: enough AA for a circle at this size

# Dark disc so the glyph reads on both the light and the dark launcher wallpaper,
# Ubuntu orange accent ring, white play triangle.
DISC = (0x20, 0x1C, 0x2B)
RING = (0xE9, 0x54, 0x20)
GLYPH = (0xFF, 0xFF, 0xFF)

C = (SIZE - 1) / 2.0          # geometric centre in pixel coordinates
R_OUT = SIZE * 0.484          # outer edge of the accent ring
R_RING = SIZE * 0.043         # ring thickness
R_DISC = R_OUT - R_RING       # dark disc ends where the ring begins


def _triangle(x, y):
    """Play triangle: apex on +x, base on -x, optically centred.

    The centroid of a triangle sits at 1/3 of its width, so a triangle centred
    on its bounding box looks shifted left; nudging it right by ~4% of the icon
    puts the visual mass back on the centre.
    """
    h = SIZE * 0.30           # half height of the base
    w = SIZE * 0.30           # apex offset from the base
    bx = C - w * 0.5 + SIZE * 0.038
    dx, dy = x - bx, y - C
    if dx < 0.0 or dx > w:
        return False
    # Linear taper from the base to the apex.
    return abs(dy) <= h * (1.0 - dx / w)


def _sample(x, y):
    """Topmost opaque layer at (x, y), or None where the icon is transparent."""
    r = math.hypot(x - C, y - C)
    if r > R_OUT:
        return None
    if _triangle(x, y):
        return GLYPH
    if r > R_DISC:
        return RING
    return DISC


def render():
    rows = []
    step = 1.0 / SS
    offs = [(i + 0.5) * step - 0.5 for i in range(SS)]   # sample offsets in [-0.5, 0.5)
    n = SS * SS
    for py in range(SIZE):
        row = bytearray()
        ys = [py + o for o in offs]
        for px in range(SIZE):
            ar = ag = ab = cov = 0
            for sx in offs:
                x = px + sx
                for y in ys:
                    c = _sample(x, y)
                    if c is None:
                        continue
                    # Accumulate premultiplied: averaging straight colour would
                    # bleed the transparent black backdrop into the edges.
                    ar += c[0]
                    ag += c[1]
                    ab += c[2]
                    cov += 1
            if cov == 0:
                row += b"\x00\x00\x00\x00"
                continue
            a = (cov * 255 + n // 2) // n
            row += bytes(((ar + cov // 2) // cov,
                          (ag + cov // 2) // cov,
                          (ab + cov // 2) // cov,
                          a))
        rows.append(row)
    return rows


def png(rows):
    def chunk(kind, payload):
        return (struct.pack(">I", len(payload)) + kind + payload
                + struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF))

    # Filter type 0 (None) per scanline: the shapes are smooth, and Paeth on top
    # of zlib buys a couple hundred bytes at this size — not worth the code.
    raw = b"".join(b"\x00" + bytes(r) for r in rows)
    ihdr = struct.pack(">IIBBBBB", SIZE, SIZE, 8, 6, 0, 0, 0)   # RGBA8, no interlace
    return (b"\x89PNG\r\n\x1a\n"
            + chunk(b"IHDR", ihdr)
            + chunk(b"IDAT", zlib.compress(raw, 9))
            + chunk(b"IEND", b""))


def main():
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    out = sys.argv[1] if len(sys.argv) > 1 else os.path.join(root, "click", "waymedia.png")
    data = png(render())
    with open(out, "wb") as fh:
        fh.write(data)
    print(f"{out}: {len(data)} bytes, {SIZE}x{SIZE} RGBA")


if __name__ == "__main__":
    main()
