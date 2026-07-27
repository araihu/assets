#!/usr/bin/env python3
"""Render a neutral v5 review sheet with native reverse and true 16 px rasters."""

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
CONCEPTS = ROOT / "concepts" / "v5"
REVIEW = ROOT / "review"
PRODUCTS = [
    ("araihu", "Arai Hû"),
    ("goshtoso", "Goshtoso"),
    ("manja", "Manja"),
    ("paje", "Pajé"),
    ("xisnove", "Xisnove"),
]
PALETTE = {"#07111f", "#31588f", "#f3f2e9", "#c7ff4a"}


def q(tag: str) -> str:
    return f"{{{SVG}}}{tag}"


def add(parent: ET.Element, tag: str, attrs: dict[str, str] | None = None, text: str | None = None) -> ET.Element:
    node = ET.SubElement(parent, q(tag), attrs or {})
    node.text = text
    return node


def artwork(path: Path, mono: bool = False) -> list[ET.Element]:
    root = ET.parse(path).getroot()
    nodes = [copy.deepcopy(node) for node in root if node.tag not in {q("title"), q("desc")}]
    if mono:
        for node in nodes:
            for item in node.iter():
                for attr in ("fill", "stroke"):
                    if item.get(attr, "").lower() in PALETTE:
                        item.set(attr, "#07111f")
    return nodes


def place(parent: ET.Element, path: Path, x: float, y: float, size: float, mono: bool = False) -> None:
    group = add(parent, "g", {"transform": f"translate({x} {y}) scale({size / 128})"})
    group.extend(artwork(path, mono=mono))


def raster_16(path: Path) -> str:
    result = subprocess.run(
        ["rsvg-convert", "--width=16", "--height=16", str(path)],
        check=True,
        stdout=subprocess.PIPE,
    )
    return "data:image/png;base64," + base64.b64encode(result.stdout).decode("ascii")


def main() -> None:
    root = ET.Element(q("svg"), {"viewBox": "0 0 1600 1760", "role": "img", "aria-labelledby": "title desc"})
    add(root, "title", {"id": "title"}, "Arai Hû logo system v5 neutral review")
    add(root, "desc", {"id": "desc"}, "Shared storm hood family tested in color, native reverse, monochrome, blind order, and actual sixteen pixel raster.")
    add(root, "rect", {"width": "1600", "height": "1760", "fill": "#d9d9d0"})

    add(root, "text", {"x": "70", "y": "88", "fill": "#07111f", "font-family": "system-ui, sans-serif", "font-size": "48", "font-weight": "800"}, "V5 · one master silhouette")
    add(root, "text", {"x": "70", "y": "126", "fill": "#34465d", "font-family": "system-ui, sans-serif", "font-size": "18"}, "Same scale. Native reverse. No reference advantage. True 16 px raster.")

    add(root, "rect", {"x": "70", "y": "160", "width": "1460", "height": "230", "fill": "#f3f2e9", "stroke": "#93a1b1"})
    add(root, "text", {"x": "94", "y": "194", "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "15", "font-weight": "700", "letter-spacing": "1.5"}, "BLIND MONOCHROME FAMILY")
    for index, (slug, _) in enumerate(PRODUCTS):
        place(root, CONCEPTS / f"{slug}-mark.svg", 132 + index * 284, 220, 128, mono=True)

    headers = [("REGULAR", 340), ("NATIVE REVERSE", 610), ("MONO", 880), ("TRUE 16 PX", 1120), ("FAVICON", 1370)]
    for label, x in headers:
        add(root, "text", {"x": str(x), "y": "438", "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "14", "font-weight": "700", "letter-spacing": "1.2", "text-anchor": "middle"}, label)

    for row, (slug, name) in enumerate(PRODUCTS):
        y = 470 + row * 238
        add(root, "rect", {"x": "70", "y": str(y), "width": "1460", "height": "218", "fill": "#f3f2e9", "stroke": "#93a1b1"})
        add(root, "text", {"x": "94", "y": str(y + 62), "fill": "#07111f", "font-family": "system-ui, sans-serif", "font-size": "24", "font-weight": "750"}, name)
        add(root, "text", {"x": "94", "y": str(y + 88), "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "14"}, slug)

        place(root, CONCEPTS / f"{slug}-mark.svg", 276, y + 36, 128)
        add(root, "rect", {"x": "546", "y": str(y + 24), "width": "128", "height": "160", "rx": "12", "fill": "#07111f"})
        place(root, CONCEPTS / f"{slug}-mark-reverse.svg", 546, y + 40, 128)
        place(root, CONCEPTS / f"{slug}-mark.svg", 816, y + 36, 128, mono=True)

        image = add(root, "image", {"x": "1072", "y": str(y + 40), "width": "96", "height": "96", "preserveAspectRatio": "none", "image-rendering": "pixelated"})
        image.set(f"{{{XLINK}}}href", raster_16(CONCEPTS / f"{slug}-mark.svg"))
        add(root, "rect", {"x": "1072", "y": str(y + 40), "width": "96", "height": "96", "fill": "none", "stroke": "#c8d1db"})
        add(root, "text", {"x": "1120", "y": str(y + 158), "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "13", "text-anchor": "middle"}, "16 × 16")

        place(root, CONCEPTS / f"{slug}-favicon.svg", 1306, y + 52, 96)

    ET.indent(root, space="  ")
    REVIEW.mkdir(exist_ok=True)
    svg_path = REVIEW / "logo-system-v5.svg"
    png_path = REVIEW / "logo-system-v5.png"
    svg_path.write_text(ET.tostring(root, encoding="unicode") + "\n", encoding="utf-8")
    subprocess.run(["rsvg-convert", "--width=1600", "--height=1760", "--output", str(png_path), str(svg_path)], check=True)
    print(png_path)


if __name__ == "__main__":
    main()
