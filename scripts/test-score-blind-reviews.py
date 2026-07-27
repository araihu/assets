#!/usr/bin/env python3

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("score-blind-reviews.py")
PRODUCTS = ("araihu", "goshtoso", "manja", "paje", "xisnove")


def payload(evaluator: str, score: int = 5) -> dict:
    return {
        "schema": 3,
        "system": "araihu-logo-system-v3",
        "evaluator": evaluator,
        "answers": {
            product: {
                "reading": f"{product} intended object",
                "clarity": str(score),
                "distinct": str(score),
                "matched": True,
            }
            for product in PRODUCTS
        },
    }


class ScoreBlindReviewsTest(unittest.TestCase):
    def run_reviews(self, evaluators: list[str], score: int = 5) -> subprocess.CompletedProcess[str]:
        with tempfile.TemporaryDirectory() as directory:
            paths = []
            for index, evaluator in enumerate(evaluators):
                path = Path(directory) / f"review-{index}.json"
                path.write_text(json.dumps(payload(evaluator, score)), encoding="utf-8")
                paths.append(path)
            return subprocess.run(
                [sys.executable, str(SCRIPT), *(str(path) for path in paths)],
                capture_output=True,
                check=False,
                text=True,
            )

    def test_five_complete_independent_reviews_pass(self) -> None:
        result = self.run_reviews([f"reviewer-{index}" for index in range(5)])
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("Quantitative result: PASS", result.stdout)

    def test_duplicate_evaluator_is_rejected(self) -> None:
        result = self.run_reviews(["same"] * 5)
        self.assertEqual(result.returncode, 2)
        self.assertIn("duplicate evaluator identifiers", result.stderr)

    def test_low_scores_fail_the_gate(self) -> None:
        result = self.run_reviews([f"reviewer-{index}" for index in range(5)], score=3)
        self.assertEqual(result.returncode, 1)
        self.assertIn("Quantitative result: FAIL", result.stdout)


if __name__ == "__main__":
    unittest.main()
