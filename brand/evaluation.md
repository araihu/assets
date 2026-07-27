# Logo quality gate

Every candidate is reviewed as a sign first and a brand asset second. A valid
SVG is necessary but not evidence of a good logo.

## Scorecard

| Criterion | Weight | Evidence |
| --- | ---: | --- |
| Immediate recognition | 25 | Unprompted viewer can name the depicted object or system at 32 px. |
| Product relevance | 25 | Explanation uses real product behavior, not an invented metaphor. |
| Small-size survival | 20 | Distinct silhouette and signal remain legible at 16 px and 32 px. |
| Family coherence | 15 | Shared geometry and palette are visible without identical containers. |
| Distinctiveness | 10 | Marks remain distinguishable in monochrome and as silhouettes. |
| Reproduction | 5 | No text, gradients, filters, masks, or fragile detail in the compact mark. |

Passing score: `80/100`, with no zero in any criterion. Recognition and product
relevance are stop gates: a candidate failing either is rejected even if its
total would otherwise pass.

## Review views

1. Paper background at 128 px.
2. Midnight background at 128 px.
3. Color at 32 px and 16 px.
4. Monochrome silhouette at 32 px.
5. Five marks in one row, labels hidden.
6. Favicon in a browser-tab simulation.

## Blind recognition protocol

Use `review/blind-review.html` with at least five people who were not shown the
brief or product mapping. Each reviewer records the first object, system, or
idea perceived before revealing names. The page prevents reveal until every
first reading and score is explicit. After reveal, the reviewer records whether
their reading belongs to the intended category and exports schema 2 JSON.

Promotion requires:

- at least four of five reviewers connect each mark to its intended visual
  category without prompting;
- mean clarity is at least `4/5`;
- mean distinction is at least `4/5`;
- no two products receive the same dominant first reading;
- raw exported JSON remains attached to the design review.

Aggregate five or more independent exports with:

```sh
scripts/score-blind-reviews.py review-results/*.json
```

The command rejects incomplete exports and duplicate evaluator identifiers,
then calculates the category-match, clarity, distinction, and exact dominant
reading gates. Synonyms in dominant readings still require one manual semantic
check before promotion.

Exact product names are not required. Arai Hû should read as storm cloud,
Manja as technical documentation, Pajé as staged route or workflow, Xisnove as
rail or signal diagram, and Goshtoso as rhythmic wave or interface flow.

## Automated checks

- XML parses successfully.
- Compact marks use `viewBox="0 0 128 128"`.
- Favicons use `viewBox="0 0 64 64"`.
- Compact marks contain no `<text>`, gradient, filter, embedded bitmap, or
  external reference.
- Colors belong to the approved palette.
- Every required product has mark, logo, and favicon before promotion.

Automated checks cannot award recognition or meaning. Those require visual
review and a written score.
