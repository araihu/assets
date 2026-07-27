#!/usr/bin/env python3
"""Render regular, native reverse, and favicon lockups for a version."""

from __future__ import annotations

import argparse
import base64
import copy
import subprocess
import xml.etree.ElementTree as ET
from pathlib import Path


SVG = "http://www.w3.org/2000/svg"
XLINK = "http://www.w3.org/1999/xlink"
ET.register_namespace("", SVG)
ET.register_namespace("xlink", XLINK)
PRODUCTS = (
    ("araihu", "Arai Hû"),
    ("goshtoso", "Goshtoso"),
    ("manja", "Manja"),
    ("paje", "Pajé"),
    ("xisnove", "Xisnove"),
)


def q(tag: str) -> str:
    return f"{{{SVG}}}{tag}"


def add(parent: ET.Element, tag: str, attrs: dict[str, str] | None = None, text: str | None = None) -> ET.Element:
    node = ET.SubElement(parent, q(tag), attrs or {})
    node.text = text
    return node


def artwork(path: Path) -> list[ET.Element]:
    root = ET.parse(path).getroot()
    return [copy.deepcopy(node) for node in root if node.tag not in {q("title"), q("desc"), q("metadata")}]


def place(parent: ET.Element, path: Path, x: float, y: float, scale: float) -> None:
    group = add(parent, "g", {"transform": f"translate({x} {y}) scale({scale})"})
    group.extend(artwork(path))


def raster_16(path: Path) -> str:
    png = subprocess.run(["rsvg-convert", "--width=16", "--height=16", str(path)], check=True, stdout=subprocess.PIPE).stdout
    return "data:image/png;base64," + base64.b64encode(png).decode("ascii")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True, help="Concept version, for example 8 or v8.")
    args = parser.parse_args()
    version = args.version if args.version.startswith("v") else f"v{args.version}"
    root_dir = Path(__file__).resolve().parent.parent
    concepts = root_dir / "concepts" / version
    review = root_dir / "review" / "screenshots"
    review.mkdir(parents=True, exist_ok=True)

    root = ET.Element(q("svg"), {"viewBox": "0 0 1800 1560", "role": "img", "aria-labelledby": "title desc"})
    add(root, "title", {"id": "title"}, f"Arai Hû {version} lockup screenshot")
    add(root, "desc", {"id": "desc"}, "Outlined wordmarks, native reverse lockups, and favicon sizes.")
    add(root, "rect", {"width": "1800", "height": "1560", "fill": "#d9d9d0"})
    add(root, "text", {"x": "70", "y": "84", "fill": "#07111f", "font-family": "system-ui, sans-serif", "font-size": "48", "font-weight": "800"}, f"{version.upper()} · lockups and favicons")
    add(root, "text", {"x": "70", "y": "120", "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "18"}, "Canonical outlined wordmarks · native reverse · actual favicon sizes")
    for label, x in (("REGULAR LOCKUP", 510), ("NATIVE REVERSE", 1130), ("FAVICON", 1600)):
        add(root, "text", {"x": str(x), "y": "174", "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "14", "font-weight": "700", "letter-spacing": "1.2", "text-anchor": "middle"}, label)

    for row, (slug, name) in enumerate(PRODUCTS):
        y = 200 + row * 258
        add(root, "rect", {"x": "70", "y": str(y), "width": "1660", "height": "234", "fill": "#f3f2e9", "stroke": "#93a1b1"})
        add(root, "text", {"x": "94", "y": str(y + 54), "fill": "#07111f", "font-family": "system-ui, sans-serif", "font-size": "22", "font-weight": "750"}, name)
        place(root, concepts / f"{slug}-logo-outlined.svg", 250, y + 42, 0.72)
        add(root, "rect", {"x": "870", "y": str(y + 22), "width": "520", "height": "190", "rx": "16", "fill": "#07111f"})
        place(root, concepts / f"{slug}-logo-outlined-reverse.svg", 880, y + 48, 0.68)
        place(root, concepts / f"{slug}-favicon.svg", 1470, y + 58, 1.0)
        place(root, concepts / f"{slug}-favicon.svg", 1560, y + 74, 0.5)
        place(root, concepts / f"{slug}-favicon.svg", 1618, y + 82, 0.25)
        image = add(root, "image", {"x": "1660", "y": str(y + 52), "width": "96", "height": "96", "preserveAspectRatio": "none", "image-rendering": "pixelated"})
        image.set(f"{{{XLINK}}}href", raster_16(concepts / f"{slug}-favicon.svg"))
        add(root, "rect", {"x": "1660", "y": str(y + 52), "width": "96", "height": "96", "fill": "none", "stroke": "#c8d1db"})
        add(root, "text", {"x": "1708", "y": str(y + 172), "fill": "#526981", "font-family": "system-ui, sans-serif", "font-size": "12", "text-anchor": "middle"}, "16 px ×6")

    ET.indent(root, space="  ")
    svg_path = review / f"logo-system-{version}-lockups.svg"
    png_path = review / f"logo-system-{version}-lockups.png"
    svg_path.write_text(ET.tostring(root, encoding="unicode") + "\n", encoding="utf-8")
    subprocess.run(["rsvg-convert", "--width=1800", "--height=1560", f"--output={png_path}", str(svg_path)], check=True)
    print(png_path)


if __name__ == "__main__":
    main()
