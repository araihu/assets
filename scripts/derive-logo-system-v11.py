#!/usr/bin/env python3
"""Build the temporary V11 calibration family from promoted vector masters."""

from __future__ import annotations

import argparse
import re
import sys
import xml.etree.ElementTree as ET
from pathlib import Path


SVG = "http://www.w3.org/2000/svg"
ET.register_namespace("", SVG)
ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT / "source" / "brand" / "original"
TARGET = ROOT / "source" / "brand" / "proof" / "v11"

PRODUCTS = {
    "araihu": "Arai Hû",
    "goshtoso": "Goshtoso",
    "manja": "Manja",
    "paje": "Pajé",
    "x9": "X-9",
}
KINDS = ("icon", "logo")
MODES = ("background", "transparent")
HEX = re.compile(r"^#([0-9a-fA-F]{6})$")

# Optical canvases measured from the approved transparent promoted masters in a
# browser SVG renderer. Path geometry stays untouched: these viewBoxes only
# normalize how much of an <img> box each sign occupies. Icons use a square
# canvas with the dominant art dimension at 76%; logos carry 8% art-relative
# padding on every side. Background variants retain their full safe canvas.
OPTICAL_VIEWBOXES = {
    ("araihu", "icon"): "262.510 263.185 498.904 498.904",
    ("goshtoso", "icon"): "237.162 237.080 550.558 550.558",
    ("manja", "icon"): "167.325 123.377 776.662 776.662",
    ("paje", "icon"): "154.532 155.566 715.652 715.652",
    ("x9", "icon"): "69.210 69.362 885.279 885.279",
    ("araihu", "logo"): "156.655 88.812 1781.202 330.139",
    ("goshtoso", "logo"): "133.328 88.739 1801.022 330.718",
    ("manja", "logo"): "304.217 91.472 1440.066 325.893",
    ("paje", "logo"): "525.286 99.476 985.121 332.227",
    ("x9", "logo"): "209.911 48.626 1620.856 410.636",
}

STYLE = """.v11-surface {
  fill: var(--araihu-logo-surface, var(--araihu-logo-auto-surface, #f3f2e9));
}
.v11-ink {
  fill: var(--araihu-logo-ink, var(--araihu-logo-auto-ink, #07111f));
}
.v11-signal {
  fill: var(--araihu-logo-signal, var(--araihu-logo-auto-signal, #c7ff4a));
}
@media (prefers-color-scheme: dark) {
  :root {
    --araihu-logo-auto-surface: #07111f;
    --araihu-logo-auto-ink: #f3f2e9;
    --araihu-logo-auto-signal: #c7ff4a;
  }
}"""


def q(tag: str) -> str:
    return f"{{{SVG}}}{tag}"


def color_role(value: str) -> str:
    match = HEX.fullmatch(value)
    if match is None:
        raise ValueError(f"unsupported color value: {value}")
    red, green, blue = bytes.fromhex(match.group(1))
    if red >= 130 and green >= 190 and blue <= 130:
        return "signal"
    if red >= 180 and green >= 180 and blue >= 180:
        return "surface"
    if red <= 80 and green <= 80 and blue <= 100:
        return "ink"
    raise ValueError(f"unclassified v11 source color: {value}")


def semanticize_colors(root: ET.Element, source: Path) -> None:
    for element in root.iter():
        for attribute in ("fill", "stroke"):
            value = element.get(attribute)
            if value is None or value == "none" or not value.startswith("#"):
                continue
            try:
                role = color_role(value)
            except ValueError as error:
                raise SystemExit(f"{source}: {error}") from error
            existing = element.get("class", "").split()
            role_class = f"v11-{role}"
            if role_class not in existing:
                existing.append(role_class)
            element.set("class", " ".join(existing))
            element.attrib.pop(attribute)

    # SVG's implicit fill is black. Make that role explicit so every geometry
    # element responds to the same consumer theme contract.
    geometry_tags = {"path", "rect", "circle", "ellipse", "polygon", "polyline"}
    for element in root.iter():
        if element.tag.rsplit("}", 1)[-1] in geometry_tags and not element.get("class"):
            element.set("class", "v11-ink")


def build_asset(product: str, name: str, kind: str, mode: str) -> str:
    source = SOURCE / f"{product}-{kind}-{mode}.svg"
    if not source.is_file():
        raise SystemExit(f"missing approved v11 source: {source}")

    root = ET.parse(source).getroot()
    root.attrib.pop("width", None)
    root.attrib.pop("height", None)
    if mode == "transparent":
        root.set("viewBox", OPTICAL_VIEWBOXES[(product, kind)])
    root.set("role", "img")
    root.set("aria-label", f"{name} v11 {kind}")
    root.set("class", "araihu-brand-v11")

    semanticize_colors(root, source)

    # Keep inline copies collision-free. Static title/desc IDs collide as soon
    # as the same asset is embedded twice in one document, so the accessible
    # name lives on the SVG root instead.
    title = ET.Element(q("title"))
    title.text = f"{name} v11 {kind}"
    desc = ET.Element(q("desc"))
    surface = "with an adaptive surface" if mode == "background" else "on a transparent surface"
    desc.text = f"Adaptive {name} {kind} {surface}; colors respond to light and dark themes."
    style = ET.Element(q("style"))
    style.text = STYLE
    root.insert(0, style)
    root.insert(0, desc)
    root.insert(0, title)

    ET.indent(root, space="  ")
    return ET.tostring(root, encoding="unicode") + "\n"


def expected_assets() -> dict[Path, str]:
    return {
        TARGET / f"{product}-{kind}-{mode}.svg": build_asset(product, name, kind, mode)
        for product, name in PRODUCTS.items()
        for kind in KINDS
        for mode in MODES
    }


def geometry_signature(path: Path) -> tuple[tuple[str, tuple[tuple[str, str], ...]], ...]:
    root = ET.parse(path).getroot()
    geometry = []
    for element in root.iter():
        tag = element.tag.rsplit("}", 1)[-1]
        if tag in {"svg", "title", "desc", "style"}:
            continue
        attributes = tuple(
            sorted(
                (key, value)
                for key, value in element.attrib.items()
                if key not in {"fill", "stroke", "class"}
            )
        )
        geometry.append((tag, attributes))
    return tuple(geometry)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true", help="fail if generated v11 assets drift")
    args = parser.parse_args()
    assets = expected_assets()

    if args.check:
        failures = []
        for path, expected in assets.items():
            if not path.is_file():
                failures.append(f"missing: {path.relative_to(ROOT)}")
            elif path.read_text(encoding="utf-8") != expected:
                failures.append(f"drift: {path.relative_to(ROOT)}")
            else:
                source = SOURCE / path.name
                if geometry_signature(source) != geometry_signature(path):
                    failures.append(f"geometry changed: {path.relative_to(ROOT)}")
        actual = set(TARGET.glob("*.svg")) if TARGET.is_dir() else set()
        failures.extend(
            f"unexpected: {path.relative_to(ROOT)}" for path in sorted(actual - set(assets))
        )
        if failures:
            print("\n".join(failures), file=sys.stderr)
            return 1
        print("v11 generated assets are current")
        return 0

    TARGET.mkdir(parents=True, exist_ok=True)
    for path, content in assets.items():
        path.write_text(content, encoding="utf-8")
    print(f"generated {len(assets)} adaptive v11 SVGs in {TARGET.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
