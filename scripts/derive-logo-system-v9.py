#!/usr/bin/env python3
"""Derive distinct V9 cloud silhouettes from the V8 charged-cloud family."""

from __future__ import annotations

import copy
import xml.etree.ElementTree as ET
from pathlib import Path


SVG = "http://www.w3.org/2000/svg"
ET.register_namespace("", SVG)
ROOT = Path(__file__).resolve().parent.parent
SOURCE = ROOT / "concepts" / "v8"
TARGET = ROOT / "concepts" / "v9"

PRODUCTS = {
    "araihu": (
        "Arai Hû",
        "M29 104C15 104 6 94 6 81s9-23 23-24c5-22 23-37 44-37 22 0 40 16 43 38 6 4 10 12 10 21 0 14-10 24-24 24H90c-7 4-13 7-20 7l-6-7-6 7c-7 0-13-2-20-6Z",
        "The master dark storm cloud with a charged lower notch and lightning cut.",
    ),
    "goshtoso": (
        "Goshtoso",
        "M22 106C10 106 3 97 3 85s9-22 22-22c6-18 23-30 42-30 20 0 37 12 44 31 9 3 14 11 14 21 0 13-9 22-22 22H88c-8 0-15 3-23 9-8-6-15-9-23-9H22Z",
        "A wide flowing derivative of the Arai Hû cloud containing four composable modules.",
    ),
    "manja": (
        "Manja",
        "M30 106C15 106 6 96 6 82s10-24 25-24c4-24 23-42 46-42 24 0 43 19 45 44 5 5 8 11 8 19 0 15-10 25-25 27H91c-10 0-18 3-27 10-9-7-17-10-27-10Z",
        "A tall publishing derivative of the Arai Hû cloud containing an open API publication.",
    ),
    "paje": (
        "Pajé",
        "M22 103C10 103 4 94 4 82s9-22 22-22c7-19 24-32 44-32 21 0 39 14 44 34 9 3 14 11 14 21 0 13-9 22-22 22H93c-9 0-16 5-23 12l-6 6-6-6c-7-7-14-12-23-12H22Z",
        "A routed derivative of the Arai Hû cloud containing a five-checkpoint durable path.",
    ),
    "xisnove": (
        "Xisnove",
        "M34 110C20 110 10 100 10 87s9-23 22-24c2-27 17-47 36-47 21 0 37 22 37 50 9 4 14 12 14 23 0 13-9 22-22 22H86l-8 8-8-8Z",
        "A tall signal derivative of the Arai Hû cloud containing three external health states.",
    ),
}


def q(tag: str) -> str:
    return f"{{{SVG}}}{tag}"


def build(slug: str, name: str, cloud_path: str, description: str) -> None:
    root = copy.deepcopy(ET.parse(SOURCE / f"{slug}-a.svg").getroot())
    title = root.find(q("title"))
    desc = root.find(q("desc"))
    paths = root.findall(q("path"))
    if title is None or desc is None or not paths:
        raise SystemExit(f"incomplete V8 source for {slug}")
    title.text = f"{name} v9 derivative cloud"
    desc.text = description
    paths[0].set("d", cloud_path)
    ET.indent(root, space="  ")
    (TARGET / f"{slug}-a.svg").write_text(ET.tostring(root, encoding="unicode") + "\n", encoding="utf-8")


def main() -> None:
    TARGET.mkdir(parents=True, exist_ok=True)
    for slug, (name, cloud_path, description) in PRODUCTS.items():
        build(slug, name, cloud_path, description)


if __name__ == "__main__":
    main()
