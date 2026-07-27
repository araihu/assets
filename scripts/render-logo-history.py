#!/usr/bin/env python3
"""Render normalized screenshots for every logo-system version and one history sheet."""

from __future__ import annotations

import base64
import copy
import subprocess
import xml.etree.ElementTree as ET
from pathlib import Path


SVG = "http://www.w3.org/2000/svg"
XLINK = "http://www.w3.org/1999/xlink"
ET.register_namespace("", SVG)
ET.register_namespace("xlink", XLINK)

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / "review" / "screenshots"
VERSIONS = ("v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9", "v10")
PRODUCTS = (
    ("araihu", "Arai Hû"),
    ("goshtoso", "Goshtoso"),
    ("manja", "Manja"),
    ("paje", "Pajé"),
    ("xisnove", "Xisnove"),
)
PALETTE = {"#07111f", "#31588f", "#c7ff4a"}


def q(tag: str) -> str:
    return f"{{{SVG}}}{tag}"


def add(parent: ET.Element, tag: str, attrs: dict[str, str] | None = None, text: str | None = None) -> ET.Element:
    node = ET.SubElement(parent, q(tag), attrs or {})
    node.text = text
    return node


def source(version: str, slug: str) -> Path:
    return ROOT / "concepts" / version / f"{slug}-mark.svg"


def artwork(path: Path, mono: bool = False) -> list[ET.Element]:
    root = ET.parse(path).getroot()
    nodes = [copy.deepcopy(node) for node in root if node.tag not in {q("title"), q("desc")}]
    if mono:
        filled_cell = any(part in {"v6", "v7", "v8", "v9"} for part in path.parts) or (
            "v10" in path.parts and path.name.startswith("araihu-")
        )
        for node in nodes:
            for item in node.iter():
                for attr in ("fill", "stroke"):
                    value = item.get(attr, "").lower()
                    if filled_cell and value in {"#31588f", "#c7ff4a"}:
                        item.set(attr, "#f3f2e9")
                    elif value in PALETTE:
                        item.set(attr, "#07111f")
    return nodes


def place(parent: ET.Element, path: Path, x: float, y: float, size: float, mono: bool = False) -> None:
    group = add(parent, "g", {"transform": f"translate({x} {y}) scale({size / 128})"})
    group.extend(artwork(path, mono=mono))


def raster_16(path: Path) -> str:
    png = subprocess.run(
        ["rsvg-convert", "--width=16", "--height=16", str(path)],
        check=True,
        stdout=subprocess.PIPE,
    ).stdout
    return "data:image/png;base64," + base64.b64encode(png).decode("ascii")


def pixel_image(parent: ET.Element, path: Path, x: int, y: int, size: int) -> None:
    image = add(parent, "image", {
        "x": str(x),
        "y": str(y),
        "width": str(size),
        "height": str(size),
        "preserveAspectRatio": "none",
        "image-rendering": "pixelated",
    })
    image.set(f"{{{XLINK}}}href", raster_16(path))
    add(parent, "rect", {"x": str(x), "y": str(y), "width": str(size), "height": str(size), "fill": "none", "stroke": "#c8d1db"})


def base(width: int, height: int, title: str, description: str) -> ET.Element:
    root = ET.Element(q("svg"), {"viewBox": f"0 0 {width} {height}", "role": "img", "aria-labelledby": "title desc"})
    add(root, "title", {"id": "title"}, title)
    add(root, "desc", {"id": "desc"}, description)
    add(root, "rect", {"width": str(width), "height": str(height), "fill": "#d9d9d0"})
    return root


def write(root: ET.Element, stem: str, width: int, height: int) -> Path:
    ET.indent(root, space="  ")
    svg_path = OUT / f"{stem}.svg"
    png_path = OUT / f"{stem}.png"
    svg_path.write_text(ET.tostring(root, encoding="unicode") + "\n", encoding="utf-8")
    subprocess.run(["rsvg-convert", f"--width={width}", f"--height={height}", f"--output={png_path}", str(svg_path)], check=True)
    return png_path


def render_version(version: str) -> Path:
    root = base(1600, 1320, f"Arai Hû logo system {version} screenshot", "Normalized color, monochrome, blind, and true sixteen pixel comparison.")
    add(root, "text", {"x": "70", "y": "86", "fill": "#07111f", "font-family": "system-ui, sans-serif", "font-size": "48", "font-weight": "800"}, f"{version.upper()} · normalized screenshot")
    add(root, "text", {"x": "70", "y": "122", "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "18"}, "Same 128 field · blind mono · color · mono · true 16 px")

    add(root, "rect", {"x": "70", "y": "156", "width": "1460", "height": "210", "fill": "#f3f2e9", "stroke": "#93a1b1"})
    add(root, "text", {"x": "92", "y": "188", "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "14", "font-weight": "700", "letter-spacing": "1.2"}, "BLIND MONOCHROME")
    for index, (slug, _) in enumerate(PRODUCTS):
        place(root, source(version, slug), 126 + index * 286, 212, 120, mono=True)

    for label, x in (("COLOR", 610), ("MONO", 910), ("TRUE 16 PX", 1210)):
        add(root, "text", {"x": str(x), "y": "408", "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "14", "font-weight": "700", "letter-spacing": "1.2", "text-anchor": "middle"}, label)

    for row, (slug, name) in enumerate(PRODUCTS):
        y = 432 + row * 170
        add(root, "rect", {"x": "70", "y": str(y), "width": "1460", "height": "152", "fill": "#f3f2e9", "stroke": "#93a1b1"})
        add(root, "text", {"x": "94", "y": str(y + 64), "fill": "#07111f", "font-family": "system-ui, sans-serif", "font-size": "24", "font-weight": "750"}, name)
        add(root, "text", {"x": "94", "y": str(y + 90), "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "13"}, slug)
        place(root, source(version, slug), 546, y + 12, 128)
        place(root, source(version, slug), 846, y + 12, 128, mono=True)
        pixel_image(root, source(version, slug), 1162, y + 28, 96)
    return write(root, f"logo-system-{version}", 1600, 1320)


def render_history() -> Path:
    root = base(3500, 1760, "Arai Hû logo system visual history", "All preserved versions in the same visual test.")
    add(root, "text", {"x": "70", "y": "86", "fill": "#07111f", "font-family": "system-ui, sans-serif", "font-size": "48", "font-weight": "800"}, f"Visual history · {VERSIONS[0]} to {VERSIONS[-1]}")
    add(root, "text", {"x": "70", "y": "122", "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "18"}, "Every mark uses identical field, scale, background, and sixteen-pixel raster test.")
    column_x = {"v2": 320, "v3": 680, "v4": 1040, "v5": 1400, "v6": 1760, "v7": 2120, "v8": 2480, "v9": 2840, "v10": 3200}
    for version, x in column_x.items():
        add(root, "text", {"x": str(x), "y": "184", "fill": "#07111f", "font-family": "system-ui, sans-serif", "font-size": "28", "font-weight": "800", "text-anchor": "middle"}, version.upper())
    for row, (slug, name) in enumerate(PRODUCTS):
        y = 220 + row * 292
        add(root, "rect", {"x": "70", "y": str(y), "width": "3360", "height": "266", "fill": "#f3f2e9", "stroke": "#93a1b1"})
        add(root, "text", {"x": "94", "y": str(y + 62), "fill": "#07111f", "font-family": "system-ui, sans-serif", "font-size": "24", "font-weight": "750"}, name)
        add(root, "text", {"x": "94", "y": str(y + 88), "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "13"}, slug)
        for version, x in column_x.items():
            add(root, "rect", {"x": str(x - 150), "y": str(y + 18), "width": "300", "height": "230", "fill": "none", "stroke": "#d0d7df"})
            place(root, source(version, slug), x - 64, y + 32, 128)
            pixel_image(root, source(version, slug), x - 32, y + 176, 64)
    return write(root, "logo-system-all-versions", 3500, 1760)


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    for version in VERSIONS:
        print(render_version(version))
    print(render_history())


if __name__ == "__main__":
    main()
