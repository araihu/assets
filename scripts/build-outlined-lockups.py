#!/usr/bin/env python3
"""Combine versioned marks with the canonical outlined wordmarks."""

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


def q(tag: str) -> str:
    return f"{{{SVG}}}{tag}"


def artwork(path: Path) -> list[ET.Element]:
    root = ET.parse(path).getroot()
    return [copy.deepcopy(node) for node in root if node.tag not in {q("title"), q("desc"), q("metadata")}]


def root_for(name: str, description: str, reverse: bool) -> ET.Element:
    root = ET.Element(q("svg"), {"viewBox": "0 0 720 192", "role": "img", "aria-labelledby": "title desc"})
    metadata = ET.SubElement(root, q("metadata"))
    metadata.text = "Wordmark outlines reused from Instrument Sans SemiCondensed Bold under SIL OFL 1.1; see LICENSES/instrument-sans-OFL.txt."
    title = ET.SubElement(root, q("title"), {"id": "title"})
    title.text = f"{name} {'reverse ' if reverse else ''}outlined logo"
    desc = ET.SubElement(root, q("desc"), {"id": "desc"})
    desc.text = description
    return root


def wordmark(canonical: Path, reverse: bool) -> ET.Element:
    groups = ET.parse(canonical).getroot().findall(q("g"))
    if len(groups) < 2:
        raise SystemExit(f"outlined wordmark group not found in {canonical}")
    group = copy.deepcopy(groups[-1])
    if reverse:
        for node in group.iter():
            if node.get("fill", "").lower() == "#07111f":
                node.set("fill", "#f3f2e9")
    return group


def build(concept_dir: Path, canonical_dir: Path, slug: str, name: str) -> None:
    mark = concept_dir / f"{slug}-mark.svg"
    reverse_mark = concept_dir / f"{slug}-mark-reverse.svg"
    description = ET.parse(mark).getroot().findtext(q("desc")) or f"{name} mark."
    canonical = canonical_dir / f"{slug}-logo.svg"

    for reverse, mark_path, suffix in (
        (False, mark, "logo-outlined"),
        (True, reverse_mark, "logo-outlined-reverse"),
    ):
        root = root_for(name, description, reverse)
        mark_group = ET.SubElement(root, q("g"), {"transform": "translate(32 32)"})
        mark_group.extend(artwork(mark_path))
        root.append(wordmark(canonical, reverse))
        ET.indent(root, space="  ")
        (concept_dir / f"{slug}-{suffix}.svg").write_text(ET.tostring(root, encoding="unicode") + "\n", encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--concept-dir", type=Path, required=True)
    parser.add_argument("--canonical-dir", type=Path, required=True)
    args = parser.parse_args()
    for slug, name in PRODUCTS.items():
        build(args.concept_dir, args.canonical_dir, slug, name)


if __name__ == "__main__":
    main()
