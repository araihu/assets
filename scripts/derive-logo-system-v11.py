#!/usr/bin/env python3
"""Build the adaptive v11 logo family from the approved Recraft vectors."""

from __future__ import annotations

import argparse
import re
import sys
import xml.etree.ElementTree as ET
from pathlib import Path


SVG = "http://www.w3.org/2000/svg"
ET.register_namespace("", SVG)
ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT / "recraft"
TARGET = ROOT / "concepts" / "v11"

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


def build_asset(product: str, name: str, kind: str, mode: str) -> str:
    source = SOURCE / f"{product}-{kind}-{mode}.svg"
    if not source.is_file():
        raise SystemExit(f"missing approved v11 source: {source}")

    root = ET.parse(source).getroot()
    root.attrib.pop("width", None)
    root.attrib.pop("height", None)
    root.set("role", "img")
    root.set("aria-labelledby", "title desc")
    root.set("class", "araihu-brand-v11")

    semanticize_colors(root, source)

    title = ET.Element(q("title"), {"id": "title"})
    title.text = f"{name} v11 {kind}"
    desc = ET.Element(q("desc"), {"id": "desc"})
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


def geometry_signature(path: Path) -> tuple[str | None, tuple[tuple[str, tuple[tuple[str, str], ...]], ...]]:
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
    return root.get("viewBox"), tuple(geometry)


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
