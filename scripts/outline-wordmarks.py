#!/usr/bin/env python3
"""Replace concept SVG wordmark text with shaped Instrument Sans outlines."""

from __future__ import annotations

import argparse
import re
from pathlib import Path

import uharfbuzz as hb
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTFont


PRODUCTS = {
    "araihu": "Arai Hû",
    "goshtoso": "Goshtoso",
    "manja": "Manja",
    "paje": "Pajé",
    "xisnove": "Xisnove",
}


def outlined_paths(font_path: Path, text: str) -> tuple[list[str], int]:
    font_bytes = font_path.read_bytes()
    hb_face = hb.Face(font_bytes)
    hb_font = hb.Font(hb_face)
    units_per_em = hb_face.upem
    hb_font.scale = (units_per_em, units_per_em)

    buffer = hb.Buffer()
    buffer.add_str(text)
    buffer.guess_segment_properties()
    hb.shape(hb_font, buffer, {"kern": True})

    ttfont = TTFont(font_path)
    glyph_set = ttfont.getGlyphSet()
    glyph_order = ttfont.getGlyphOrder()

    cursor = 0
    paths: list[str] = []
    for info, position in zip(buffer.glyph_infos, buffer.glyph_positions, strict=True):
        glyph = glyph_set[glyph_order[info.codepoint]]
        pen = SVGPathPen(glyph_set)
        translated = TransformPen(
            pen,
            (1, 0, 0, 1, cursor + position.x_offset, position.y_offset),
        )
        glyph.draw(translated)
        command = pen.getCommands()
        if command:
            paths.append(command)
        cursor += position.x_advance
    return paths, units_per_em


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--font", type=Path, required=True)
    parser.add_argument("--concept-dir", type=Path, required=True)
    args = parser.parse_args()

    for slug, name in PRODUCTS.items():
        source = args.concept_dir / f"{slug}-logo.svg"
        target = args.concept_dir / f"{slug}-logo-outlined.svg"
        svg = source.read_text(encoding="utf-8")
        paths, units_per_em = outlined_paths(args.font, name)
        scale = 78 / units_per_em
        path_markup = "".join(f'<path d="{path}"/>' for path in paths)
        outlined = (
            f'<g transform="translate(198 123) scale({scale:.6f} -{scale:.6f})" '
            f'fill="#07111f">{path_markup}</g>'
        )
        replaced, count = re.subn(r"<text\b[^>]*>.*?</text>", outlined, svg, count=1)
        if count != 1:
            raise SystemExit(f"expected one text wordmark in {source}")
        metadata = (
            "<metadata>Wordmark outlines generated from Instrument Sans "
            "SemiCondensed Bold under SIL OFL 1.1; see LICENSES/instrument-sans-OFL.txt."
            "</metadata>"
        )
        replaced = re.sub(r"(<svg\b[^>]*>)", rf"\1{metadata}", replaced, count=1)
        target.write_text(replaced.rstrip() + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
