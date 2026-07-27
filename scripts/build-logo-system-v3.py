#!/usr/bin/env python3
"""Build the selected v3 mark, reverse, favicon, and text lockup sources."""

from __future__ import annotations

import argparse
import copy
import xml.etree.ElementTree as ET
from pathlib import Path


SVG = "http://www.w3.org/2000/svg"
ET.register_namespace("", SVG)

PRODUCTS = {
    "araihu": "Arai Hû",
    "goshtoso": "Goshtoso",
    "manja": "Manja",
    "paje": "Pajé",
    "xisnove": "Xisnove",
}


def element(tag: str, attributes: dict[str, str] | None = None) -> ET.Element:
    return ET.Element(f"{{{SVG}}}{tag}", attributes or {})


def write(path: Path, root: ET.Element) -> None:
    ET.indent(root, space="  ")
    path.write_text(ET.tostring(root, encoding="unicode") + "\n", encoding="utf-8")


def artwork(root: ET.Element) -> list[ET.Element]:
    return [
        copy.deepcopy(child)
        for child in root
        if child.tag not in {f"{{{SVG}}}title", f"{{{SVG}}}desc"}
    ]


def titled_root(view_box: str, title: str, description: str) -> ET.Element:
    root = element(
        "svg",
        {
            "viewBox": view_box,
            "role": "img",
            "aria-labelledby": "title desc",
        },
    )
    title_node = element("title", {"id": "title"})
    title_node.text = title
    root.append(title_node)
    desc_node = element("desc", {"id": "desc"})
    desc_node.text = description
    root.append(desc_node)
    return root


def replace_midnight_with_paper(root: ET.Element) -> None:
    for node in root.iter():
        for attribute in ("fill", "stroke"):
            if node.get(attribute, "").lower() == "#07111f":
                node.set(attribute, "#f3f2e9")


def build_product(concept_dir: Path, slug: str, name: str) -> None:
    source = concept_dir / f"{slug}-a.svg"
    source_root = ET.parse(source).getroot()
    source_desc = source_root.findtext(f"{{{SVG}}}desc") or f"{name} v3 selected mark."

    mark = titled_root("0 0 128 128", f"{name} mark", source_desc)
    mark.extend(artwork(source_root))
    write(concept_dir / f"{slug}-mark.svg", mark)

    reverse = copy.deepcopy(mark)
    reverse.find(f"{{{SVG}}}title").text = f"{name} reverse mark"
    replace_midnight_with_paper(reverse)
    write(concept_dir / f"{slug}-mark-reverse.svg", reverse)

    favicon = titled_root("0 0 64 64", f"{name} favicon", source_desc)
    favicon.append(element("rect", {"width": "64", "height": "64", "rx": "12", "fill": "#07111f"}))
    favicon_art = element("g", {"transform": "scale(.5)"})
    favicon_art.extend(artwork(reverse))
    favicon.append(favicon_art)
    write(concept_dir / f"{slug}-favicon.svg", favicon)

    logo = titled_root("0 0 720 192", f"{name} logo", source_desc)
    logo_art = element("g", {"transform": "translate(32 32)"})
    logo_art.extend(artwork(mark))
    logo.append(logo_art)
    wordmark = element(
        "text",
        {
            "x": "198",
            "y": "123",
            "fill": "#07111f",
            "font-family": "Instrument Sans SemiCondensed, sans-serif",
            "font-size": "78",
            "font-weight": "700",
        },
    )
    wordmark.text = name
    logo.append(wordmark)
    write(concept_dir / f"{slug}-logo.svg", logo)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--concept-dir", type=Path, required=True)
    args = parser.parse_args()
    for slug, name in PRODUCTS.items():
        build_product(args.concept_dir, slug, name)


if __name__ == "__main__":
    main()
