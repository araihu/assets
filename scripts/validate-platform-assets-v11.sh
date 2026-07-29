#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
target="$root/dist/v11"

python3 "$root/scripts/build-platform-assets-v11.py" --check

for product in araihu goshtoso manja paje x9; do
  web="$target/web/$product"
  test -f "$web/favicon.svg"
  xmllint --noout "$web/favicon.svg"
  python3 -m json.tool "$web/manifest-icons.json" >/dev/null
  grep -q '"purpose": "any"' "$web/manifest-icons.json"
  grep -q '"purpose": "maskable"' "$web/manifest-icons.json"

  android="$target/android/$product/res"
  for xml in "$android"/mipmap-anydpi-v26/*.xml "$android"/mipmap-anydpi-v33/*.xml "$android"/values/*.xml; do
    xmllint --noout "$xml"
  done
  grep -q '<monochrome ' "$android/mipmap-anydpi-v33/ic_launcher.xml"
  if grep -q '<monochrome ' "$android/mipmap-anydpi-v26/ic_launcher.xml"; then
    printf 'API 33 monochrome element leaked into v26 resource: %s\n' "$product" >&2
    exit 1
  fi

  apple="$target/apple/$product/Assets.xcassets/AppIcon.appiconset"
  python3 -m json.tool "$apple/Contents.json" >/dev/null
  grep -q '"value": "dark"' "$apple/Contents.json"
  grep -q '"value": "tinted"' "$apple/Contents.json"
  for image in "$apple"/*.png; do
    dimensions=$(sips -g pixelWidth -g pixelHeight "$image" 2>/dev/null)
    printf '%s' "$dimensions" | grep -q 'pixelWidth: 1024'
    printf '%s' "$dimensions" | grep -q 'pixelHeight: 1024'
    if sips -g hasAlpha "$image" 2>/dev/null | grep -q 'hasAlpha: yes'; then
      printf 'Apple app icon has an alpha channel: %s\n' "$image" >&2
      exit 1
    fi
  done
done

python3 - "$target" <<'PY'
import json
import struct
import sys
import zlib
from pathlib import Path

target = Path(sys.argv[1])
safe_ratio = 66 / 108


def decode_png(path):
    data = path.read_bytes()
    offset = 8
    compressed = bytearray()
    while offset < len(data):
        length = struct.unpack(">I", data[offset:offset + 4])[0]
        kind = data[offset + 4:offset + 8]
        payload = data[offset + 8:offset + 8 + length]
        offset += length + 12
        if kind == b"IHDR":
            width, height, depth, color_type, _, _, interlace = struct.unpack(">IIBBBBB", payload)
            if depth != 8 or color_type not in (2, 6) or interlace:
                raise SystemExit(f"unsupported PNG encoding: {path}")
        elif kind == b"IDAT":
            compressed.extend(payload)
        elif kind == b"IEND":
            break
    channels = 3 if color_type == 2 else 4
    stride = width * channels
    raw = zlib.decompress(bytes(compressed))
    rows, previous, cursor = [], bytearray(stride), 0
    for _ in range(height):
        filter_type = raw[cursor]
        cursor += 1
        source = raw[cursor:cursor + stride]
        cursor += stride
        row = bytearray(stride)
        for index, value in enumerate(source):
            left = row[index - channels] if index >= channels else 0
            above = previous[index]
            upper_left = previous[index - channels] if index >= channels else 0
            if filter_type == 0:
                predictor = 0
            elif filter_type == 1:
                predictor = left
            elif filter_type == 2:
                predictor = above
            elif filter_type == 3:
                predictor = (left + above) // 2
            elif filter_type == 4:
                estimate = left + above - upper_left
                distances = (abs(estimate - left), abs(estimate - above), abs(estimate - upper_left))
                predictor = (left, above, upper_left)[distances.index(min(distances))]
            else:
                raise SystemExit(f"unsupported PNG filter: {path}: {filter_type}")
            row[index] = (value + predictor) & 255
        rows.append(row)
        previous = row
    return width, height, channels, rows


def assert_safe(path, background=None):
    width, height, channels, rows = decode_png(path)
    points = []
    for y, row in enumerate(rows):
        for x in range(width):
            pixel = tuple(row[x * channels:(x + 1) * channels])
            visible = pixel[3] != 0 if channels == 4 else pixel[:3] != background
            if visible:
                points.append((x, y))
    if not points:
        raise SystemExit(f"no visible art in safe-area export: {path}")
    x1, y1 = min(x for x, _ in points), min(y for _, y in points)
    x2, y2 = max(x for x, _ in points), max(y for _, y in points)
    margin = width * (1 - safe_ratio) / 2
    tolerance = 2
    if min(x1, y1) < margin - tolerance or max(x2, y2) > width - margin + tolerance:
        raise SystemExit(f"art exceeds 66/108 safe square: {path}: {(x1, y1, x2, y2)}")


expected = {
    "favicon-16.png": 16, "favicon-32.png": 32, "icon-192.png": 192,
    "icon-512.png": 512, "icon-maskable-192.png": 192,
    "icon-maskable-512.png": 512, "apple-touch-icon-180.png": 180,
}
for product in ("araihu", "goshtoso", "manja", "paje", "x9"):
    web = target / "web" / product
    for name, size in expected.items():
        data = (web / name).read_bytes()[:24]
        width, height = struct.unpack(">II", data[16:24])
        if (width, height) != (size, size):
            raise SystemExit(f"wrong PNG dimensions: {product}/{name}: {width}x{height}")
    icons = json.loads((web / "manifest-icons.json").read_text())["icons"]
    if len(icons) != 4:
        raise SystemExit(f"manifest fragment must contain four PWA icons: {product}")
    for size in (192, 512):
        assert_safe(web / f"icon-maskable-{size}.png", (243, 242, 233))

    android = target / "android" / product / "res" / "drawable-xxxhdpi"
    assert_safe(android / "ic_launcher_foreground.png")
    assert_safe(android / "ic_launcher_monochrome.png")

    apple = target / "apple" / product / "Assets.xcassets" / "AppIcon.appiconset"
    assert_safe(apple / "AppIcon-1024.png", (243, 242, 233))
    assert_safe(apple / "AppIcon-1024-dark.png", (7, 17, 31))
    assert_safe(apple / "AppIcon-1024-tinted.png", (230, 230, 230))
PY

printf 'v11 platform asset gates passed\n'
