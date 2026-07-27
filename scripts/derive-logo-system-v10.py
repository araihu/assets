#!/usr/bin/env python3
"""Build V10: one Arai Hû cloud and four standalone product signs."""

from __future__ import annotations

import copy
import xml.etree.ElementTree as ET
from pathlib import Path


SVG = "http://www.w3.org/2000/svg"
ET.register_namespace("", SVG)
ROOT = Path(__file__).resolve().parent.parent
TARGET = ROOT / "concepts" / "v10"

PRODUCTS = {
    "araihu": ("Arai Hû", "v8", "The Arai Hû master dark cloud with one charged lightning cut."),
    "manja": ("Manja", "v4", "An open API publication with one routed live endpoint."),
    "paje": ("Pajé", "v4", "A continuous durable route passing through five workflow checkpoints."),
    "xisnove": ("Xisnove", "v4", "A three-state monitoring signal emitting one external probe."),
}


def q(tag: str) -> str:
    return f"{{{SVG}}}{tag}"


def add(root: ET.Element, tag: str, attributes: dict[str, str]) -> ET.Element:
    return ET.SubElement(root, q(tag), attributes)


def copied_product(slug: str, name: str, version: str, description: str) -> None:
    source_suffix = "a" if version == "v8" else "mark"
    root = copy.deepcopy(ET.parse(ROOT / "concepts" / version / f"{slug}-{source_suffix}.svg").getroot())
    title = root.find(q("title"))
    desc = root.find(q("desc"))
    if title is None or desc is None:
        raise SystemExit(f"missing accessible label in {slug} {version}")
    title.text = f"{name} v10 standalone sign"
    desc.text = description
    ET.indent(root, space="  ")
    (TARGET / f"{slug}-a.svg").write_text(ET.tostring(root, encoding="unicode") + "\n", encoding="utf-8")


def goshtoso() -> None:
    root = ET.Element(q("svg"), {"viewBox": "0 0 128 128", "role": "img", "aria-labelledby": "title desc"})
    title = add(root, "title", {"id": "title"})
    title.text = "Goshtoso v10 standalone sign"
    desc = add(root, "desc", {"id": "desc"})
    desc.text = "Two overlapping rendered component panels joined by one live module."
    add(root, "rect", {"x": "10", "y": "20", "width": "72", "height": "62", "rx": "18", "fill": "none", "stroke": "#07111f", "stroke-width": "14", "stroke-linejoin": "round"})
    add(root, "rect", {"x": "46", "y": "46", "width": "72", "height": "62", "rx": "18", "fill": "none", "stroke": "#31588f", "stroke-width": "14", "stroke-linejoin": "round"})
    add(root, "rect", {"x": "52", "y": "52", "width": "24", "height": "24", "rx": "7", "fill": "#c7ff4a", "stroke": "#07111f", "stroke-width": "6"})
    ET.indent(root, space="  ")
    (TARGET / "goshtoso-a.svg").write_text(ET.tostring(root, encoding="unicode") + "\n", encoding="utf-8")


def main() -> None:
    TARGET.mkdir(parents=True, exist_ok=True)
    for slug, (name, version, description) in PRODUCTS.items():
        copied_product(slug, name, version, description)
    goshtoso()


if __name__ == "__main__":
    main()
