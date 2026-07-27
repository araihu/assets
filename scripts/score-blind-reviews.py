#!/usr/bin/env python3
"""Aggregate independent Arai Hû logo blind-review exports."""

from __future__ import annotations

import argparse
import json
import math
import re
import sys
import unicodedata
from collections import Counter
from pathlib import Path


PRODUCTS = {
    "araihu": "Arai Hû",
    "goshtoso": "Goshtoso",
    "manja": "Manja",
    "paje": "Pajé",
    "xisnove": "Xisnove",
}


def normalized_reading(value: str) -> str:
    plain = "".join(
        char
        for char in unicodedata.normalize("NFKD", value.casefold())
        if not unicodedata.combining(char)
    )
    return re.sub(r"[^a-z0-9]+", " ", plain).strip()


def score(value: object, field: str, path: Path, product: str) -> float:
    try:
        parsed = float(value)
    except (TypeError, ValueError) as error:
        raise ValueError(f"{path}: {product}.{field} is not numeric") from error
    if not 1 <= parsed <= 5:
        raise ValueError(f"{path}: {product}.{field} must be between 1 and 5")
    return parsed


def load_review(path: Path) -> dict[str, object]:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise ValueError(f"{path}: cannot read valid JSON: {error}") from error
    if payload.get("schema") != 2:
        raise ValueError(f"{path}: expected schema 2 export")
    evaluator = str(payload.get("evaluator", "")).strip()
    if not evaluator:
        raise ValueError(f"{path}: evaluator is required")
    answers = payload.get("answers")
    if not isinstance(answers, dict):
        raise ValueError(f"{path}: answers object is required")

    parsed_answers = {}
    for product in PRODUCTS:
        answer = answers.get(product)
        if not isinstance(answer, dict):
            raise ValueError(f"{path}: missing answer for {product}")
        reading = str(answer.get("reading", "")).strip()
        if not reading:
            raise ValueError(f"{path}: {product}.reading is required")
        matched = answer.get("matched")
        if not isinstance(matched, bool):
            raise ValueError(f"{path}: {product}.matched must be true or false")
        parsed_answers[product] = {
            "reading": reading,
            "clarity": score(answer.get("clarity"), "clarity", path, product),
            "distinct": score(answer.get("distinct"), "distinct", path, product),
            "matched": matched,
        }
    return {"evaluator": evaluator, "answers": parsed_answers}


def aggregate(paths: list[Path]) -> tuple[str, bool]:
    if len(paths) < 5:
        raise ValueError("at least five independent review exports are required")
    reviews = [load_review(path) for path in paths]
    evaluators = [review["evaluator"] for review in reviews]
    duplicates = sorted(name for name, count in Counter(evaluators).items() if count > 1)
    if duplicates:
        raise ValueError(f"duplicate evaluator identifiers: {', '.join(duplicates)}")

    required_matches = math.ceil(len(reviews) * 0.8)
    rows = []
    dominant_by_product = {}
    passed = True
    for product, name in PRODUCTS.items():
        answers = [review["answers"][product] for review in reviews]
        matches = sum(answer["matched"] for answer in answers)
        clarity = sum(answer["clarity"] for answer in answers) / len(answers)
        distinct = sum(answer["distinct"] for answer in answers) / len(answers)
        readings = Counter(normalized_reading(answer["reading"]) for answer in answers)
        dominant, dominant_count = readings.most_common(1)[0]
        dominant_by_product[product] = dominant
        product_passed = matches >= required_matches and clarity >= 4 and distinct >= 4
        passed = passed and product_passed
        rows.append(
            f"| {name} | {matches}/{len(reviews)} | {clarity:.2f} | {distinct:.2f} | "
            f"{dominant} ({dominant_count}) | {'PASS' if product_passed else 'FAIL'} |"
        )

    duplicated_readings = sorted(
        reading for reading, count in Counter(dominant_by_product.values()).items() if count > 1
    )
    if duplicated_readings:
        passed = False

    lines = [
        f"Reviewers: {len(reviews)} · required matches per product: {required_matches}",
        "",
        "| Product | Intended-category matches | Clarity mean | Distinction mean | Exact dominant reading | Gate |",
        "| --- | ---: | ---: | ---: | --- | --- |",
        *rows,
        "",
    ]
    if duplicated_readings:
        lines.append("FAIL: products share an exact normalized dominant reading: " + ", ".join(duplicated_readings))
    else:
        lines.append("Exact dominant readings are unique. Review semantic synonyms manually before promotion.")
    lines.append("Quantitative result: " + ("PASS" if passed else "FAIL"))
    return "\n".join(lines), passed


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("reviews", nargs="+", type=Path, help="schema 2 JSON exports")
    args = parser.parse_args()
    try:
        report, passed = aggregate(args.reviews)
    except ValueError as error:
        print(f"error: {error}", file=sys.stderr)
        return 2
    print(report)
    return 0 if passed else 1


if __name__ == "__main__":
    raise SystemExit(main())
