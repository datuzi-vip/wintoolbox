# Generate multi-size Windows-style .ico without Pillow
import struct
import zlib
import os

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "assets", "app.ico")
os.makedirs(os.path.dirname(OUT), exist_ok=True)


def make_rgba(size: int):
    gap = max(1, size // 12)
    pane = (size - gap) // 2
    px = [(0, 0, 0, 0)] * (size * size)

    def fill(x0, y0, w, h, rgba):
        for y in range(y0, min(y0 + h, size)):
            row = y * size
            for x in range(x0, min(x0 + w, size)):
                px[row + x] = rgba

    fill(0, 0, pane, pane, (0xF2, 0x50, 0x22, 0xFF))
    fill(pane + gap, 0, size - (pane + gap), pane, (0x7F, 0xBA, 0x00, 0xFF))
    fill(0, pane + gap, pane, size - (pane + gap), (0x00, 0xA4, 0xEF, 0xFF))
    fill(pane + gap, pane + gap, size - (pane + gap), size - (pane + gap), (0xFF, 0xB9, 0x00, 0xFF))
    return px


def write_png(size: int, px):
    def chunk(tag: bytes, data: bytes) -> bytes:
        return struct.pack(">I", len(data)) + tag + data + struct.pack(">I", zlib.crc32(tag + data) & 0xFFFFFFFF)

    raw = b"".join(
        b"\x00" + b"".join(bytes(px[y * size + x]) for x in range(size))
        for y in range(size)
    )
    return (
        b"\x89PNG\r\n\x1a\n"
        + chunk(b"IHDR", struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0))
        + chunk(b"IDAT", zlib.compress(raw, 9))
        + chunk(b"IEND", b"")
    )


sizes = [16, 32, 48, 64, 128, 256]
images = [(s, write_png(s, make_rgba(s))) for s in sizes]

num = len(images)
header = struct.pack("<HHH", 0, 1, num)
entries = b""
offset = 6 + 16 * num
data = b""
for s, png in images:
    w = 0 if s >= 256 else s
    h = 0 if s >= 256 else s
    entries += struct.pack("<BBBBHHII", w, h, 0, 0, 1, 32, len(png), offset)
    data += png
    offset += len(png)

with open(OUT, "wb") as f:
    f.write(header + entries + data)

print(f"wrote {OUT} ({os.path.getsize(OUT)} bytes)")
