#!/usr/bin/env python3
"""Build deterministic web, Android, and Apple packages from v11 SVG icons."""

from __future__ import annotations

import argparse
import copy
import json
import shutil
import struct
import subprocess
import sys
import tempfile
import xml.etree.ElementTree as ET
from pathlib import Path


SVG = "http://www.w3.org/2000/svg"
ET.register_namespace("", SVG)
ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT / "concepts" / "v11"
TARGET = ROOT / "dist" / "v11"
PRODUCTS = ("araihu", "goshtoso", "manja", "paje", "x9")
SAFE_ART_RATIO = 66 / 108
OPTICAL_ART_RATIO = 0.76
PALETTES = {
    "light": {"surface": "#f3f2e9", "ink": "#07111f", "signal": "#c7ff4a"},
    "dark": {"surface": "#07111f", "ink": "#f3f2e9", "signal": "#c7ff4a"},
    "tinted": {"surface": "#e6e6e6", "ink": "#202020", "signal": "#707070"},
    "mono": {"surface": "none", "ink": "#000000", "signal": "#000000"},
}


def q(tag: str) -> str:
    return f"{{{SVG}}}{tag}"


def icon_geometry(product: str) -> tuple[list[ET.Element], tuple[float, float, float, float]]:
    source = SOURCE / f"{product}-icon-transparent.svg"
    root = ET.parse(source).getroot()
    viewbox = tuple(float(value) for value in root.get("viewBox", "").split())
    if len(viewbox) != 4 or viewbox[2] != viewbox[3]:
        raise SystemExit(f"transparent icon must have a square viewBox: {source}")
    ignored = {"title", "desc", "style"}
    geometry = [copy.deepcopy(child) for child in root if child.tag.rsplit("}", 1)[-1] not in ignored]
    return geometry, viewbox


def rendered_svg(product: str, palette: str, safe: bool, background: bool) -> str:
    geometry, (x, y, width, height) = icon_geometry(product)
    if safe:
        expanded = width * OPTICAL_ART_RATIO / SAFE_ART_RATIO
        x -= (expanded - width) / 2
        y -= (expanded - height) / 2
        width = height = expanded
    root = ET.Element(q("svg"), {"viewBox": f"{x:.6f} {y:.6f} {width:.6f} {height:.6f}"})
    colors = PALETTES[palette]
    if background:
        root.append(ET.Element(q("rect"), {
            "x": f"{x:.6f}", "y": f"{y:.6f}", "width": f"{width:.6f}",
            "height": f"{height:.6f}", "fill": colors["surface"],
        }))
    style = ET.Element(q("style"))
    style.text = "\n".join(
        f".v11-{role} {{ fill: {color}; }}" for role, color in colors.items() if color != "none"
    )
    root.append(style)
    root.extend(geometry)
    return ET.tostring(root, encoding="unicode") + "\n"


def render(svg: str, output: Path, size: int, opaque: str | None = None) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(suffix=".svg") as source:
        source.write(svg.encode())
        source.flush()
        command = ["rsvg-convert", "--width", str(size), "--height", str(size), "--output", str(output)]
        if opaque:
            command.extend(["--background-color", opaque])
        command.append(source.name)
        subprocess.run(command, check=True)


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def build_web(product: str, root: Path) -> None:
    output = root / "web" / product
    output.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(SOURCE / f"{product}-icon-transparent.svg", output / "favicon.svg")
    transparent = rendered_svg(product, "light", safe=False, background=False)
    maskable = rendered_svg(product, "light", safe=True, background=True)
    for size in (16, 32):
        render(transparent, output / f"favicon-{size}.png", size)
    for size in (192, 512):
        render(transparent, output / f"icon-{size}.png", size)
        render(maskable, output / f"icon-maskable-{size}.png", size, PALETTES["light"]["surface"])
    render(maskable, output / "apple-touch-icon-180.png", 180, PALETTES["light"]["surface"])
    write_json(output / "manifest-icons.json", {"icons": [
        {"src": f"icon-{size}.png", "sizes": f"{size}x{size}", "type": "image/png", "purpose": "any"}
        for size in (192, 512)
    ] + [
        {"src": f"icon-maskable-{size}.png", "sizes": f"{size}x{size}", "type": "image/png", "purpose": "maskable"}
        for size in (192, 512)
    ]})


def adaptive_xml(monochrome: bool) -> str:
    mono = '\n  <monochrome android:drawable="@drawable/ic_launcher_monochrome" />' if monochrome else ""
    return (
        '<?xml version="1.0" encoding="utf-8"?>\n'
        '<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">\n'
        '  <background android:drawable="@color/ic_launcher_background" />\n'
        '  <foreground android:drawable="@drawable/ic_launcher_foreground" />'
        f'{mono}\n</adaptive-icon>\n'
    )


def build_android(product: str, root: Path) -> None:
    output = root / "android" / product / "res"
    foreground = rendered_svg(product, "light", safe=True, background=False)
    monochrome = rendered_svg(product, "mono", safe=True, background=False)
    opaque = rendered_svg(product, "light", safe=True, background=True)
    render(foreground, output / "drawable-xxxhdpi" / "ic_launcher_foreground.png", 432)
    render(monochrome, output / "drawable-xxxhdpi" / "ic_launcher_monochrome.png", 432)
    densities = {"mdpi": 48, "hdpi": 72, "xhdpi": 96, "xxhdpi": 144, "xxxhdpi": 192}
    for density, size in densities.items():
        render(opaque, output / f"mipmap-{density}" / "ic_launcher.png", size, PALETTES["light"]["surface"])
        render(opaque, output / f"mipmap-{density}" / "ic_launcher_round.png", size, PALETTES["light"]["surface"])
    for api, mono in ((26, False), (33, True)):
        directory = output / f"mipmap-anydpi-v{api}"
        directory.mkdir(parents=True, exist_ok=True)
        for name in ("ic_launcher.xml", "ic_launcher_round.xml"):
            (directory / name).write_text(adaptive_xml(mono), encoding="utf-8")
    values = output / "values"
    values.mkdir(parents=True, exist_ok=True)
    (values / "colors.xml").write_text(
        '<?xml version="1.0" encoding="utf-8"?>\n<resources>\n'
        '  <color name="ic_launcher_background">#F3F2E9</color>\n</resources>\n',
        encoding="utf-8",
    )


def build_apple(product: str, root: Path) -> None:
    catalog = root / "apple" / product / "Assets.xcassets"
    appicon = catalog / "AppIcon.appiconset"
    appicon.mkdir(parents=True, exist_ok=True)
    write_json(catalog / "Contents.json", {"info": {"author": "araihu", "version": 1}})
    variants = (("AppIcon-1024.png", "light", None), ("AppIcon-1024-dark.png", "dark", "dark"),
                ("AppIcon-1024-tinted.png", "tinted", "tinted"))
    images = []
    for filename, palette, appearance in variants:
        surface = PALETTES[palette]["surface"]
        render(rendered_svg(product, palette, safe=True, background=True), appicon / filename, 1024, surface)
        item: dict[str, object] = {
            "filename": filename, "idiom": "universal", "platform": "ios", "size": "1024x1024"
        }
        if appearance:
            item["appearances"] = [{"appearance": "luminosity", "value": appearance}]
        images.append(item)
    write_json(appicon / "Contents.json", {"images": images, "info": {"author": "araihu", "version": 1}})


def read_png_size(path: Path) -> tuple[int, int]:
    data = path.read_bytes()[:24]
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise SystemExit(f"not a PNG: {path}")
    return struct.unpack(">II", data[16:24])


def build(root: Path) -> None:
    for product in PRODUCTS:
        build_web(product, root)
        build_android(product, root)
        build_apple(product, root)
    (root / "README.md").write_text(
        "# v11 platform exports\n\nGenerated from `concepts/v11/`; do not edit by hand.\n\n"
        "- `web/`: favicon, PWA any/maskable icons, manifest icon fragments, and Apple touch icons.\n"
        "- `android/`: adaptive icon XML, safe-zone foregrounds, monochrome icons, and legacy density fallbacks.\n"
        "- `apple/`: iOS/iPadOS asset catalogs with 1024 px light, dark, and grayscale tinted variants.\n\n"
        "All maskable and native artwork fits the stricter Android 66/108 safe square.\n",
        encoding="utf-8",
    )


def compare(expected: Path, actual: Path) -> list[str]:
    expected_files = {path.relative_to(expected) for path in expected.rglob("*") if path.is_file()}
    actual_files = {path.relative_to(actual) for path in actual.rglob("*") if path.is_file()} if actual.is_dir() else set()
    failures = [f"missing: dist/v11/{path}" for path in sorted(expected_files - actual_files)]
    failures += [f"unexpected: dist/v11/{path}" for path in sorted(actual_files - expected_files)]
    for relative in sorted(expected_files & actual_files):
        if (expected / relative).read_bytes() != (actual / relative).read_bytes():
            failures.append(f"drift: dist/v11/{relative}")
    return failures


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="fail when generated platform exports drift")
    args = parser.parse_args()
    if shutil.which("rsvg-convert") is None:
        raise SystemExit("rsvg-convert is required")
    if args.check:
        with tempfile.TemporaryDirectory(prefix="araihu-v11-") as directory:
            expected = Path(directory) / "v11"
            build(expected)
            failures = compare(expected, TARGET)
        if failures:
            print("\n".join(failures), file=sys.stderr)
            return 1
        print("v11 platform exports are current")
        return 0
    if TARGET.exists():
        shutil.rmtree(TARGET)
    build(TARGET)
    pngs = list(TARGET.rglob("*.png"))
    if any(width != height for width, height in map(read_png_size, pngs)):
        raise SystemExit("generated a non-square platform PNG")
    print(f"generated {len(pngs)} platform PNGs for {len(PRODUCTS)} products")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
