#!/usr/bin/env python3
"""Pack the rendered icon PNGs into a multi-size Windows .ico.

Input:  ../icon-256.png / ../icon-48.png / ../icon-32.png / ../icon-16.png
        (i.e. build/ — the canonical icon directory)
Output: windows/icon.ico (Vista+ PNG-compressed entries)

The 256px frame MUST be a PNG payload: the legacy BMP-in-ICO encoding cannot
express 256x256 and Windows silently ignores such an entry.

The icons are derived from the project artwork; the raster master is
build/appicon.png (1024x1024). See AGENTS.md convention 9d for regenerating
the whole set. After packing, refresh the .syso resources linked into the
desktop executable:

  go run github.com/tc-hib/go-winres@v0.3.3 make --in build/winres.json --out cmd/desktop/rsrc

Without that step the .ico is updated but the desktop exe keeps whatever icon
was compiled into its .syso.
"""
import os
import struct
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
# The PNGs live in build/, one level up. This used to be HERE, which could
# never work: the committed PNGs were never inside build/windows/.
SRC = os.path.dirname(HERE)
SIZES = [(256, "icon-256.png"), (48, "icon-48.png"), (32, "icon-32.png"), (16, "icon-16.png")]


def main() -> None:
    blobs = []
    for size, name in SIZES:
        path = os.path.join(SRC, name)
        with open(path, "rb") as f:
            blob = f.read()
        if blob[:8] != b"\x89PNG\r\n\x1a\n":
            sys.exit(f"{name} is not a PNG")
        blobs.append((size, blob))

    header = struct.pack("<HHH", 0, 1, len(blobs))
    entries = b""
    offset = 6 + 16 * len(blobs)
    data = b""
    for size, blob in blobs:
        # ICONDIRENTRY: width%256, height%256, colors, reserved, planes, bpp, size, offset
        entries += struct.pack("<BBBBHHII", size % 256, size % 256, 0, 0, 1, 32, len(blob), offset)
        data += blob
        offset += len(blob)

    # HERE is already build/windows, so the output name must not re-append
    # "windows" — doing so wrote build/windows/windows/icon.ico.
    out = os.path.join(HERE, "icon.ico")
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "wb") as f:
        f.write(header + entries + data)
    print(f"wrote {out} ({os.path.getsize(out)} bytes, {len(blobs)} sizes)")


if __name__ == "__main__":
    main()
