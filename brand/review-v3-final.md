# V3 final design review

Canonical visual evidence: `review/logo-system-v3.png`.

## Decision

Lane A is selected for all five products. Lane B is rejected: Arai Hû B reads
as stacked food or a lamp, Pajé B reads as a mouse or gauge, and the remaining
lane-B signs weaken family recognition. Selecting one lane also preserves a
coherent artifact vocabulary across the system.

| Mark | Recognition | Relevance | Small size | Family | Distinct | Reproduction | Total | Decision |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Arai Hû | 24/25 | 25/25 | 19/20 | 14/15 | 8/10 | 5/5 | 95 | Promote. Black storm front remains immediate at 16 px. |
| Goshtoso | 20/25 | 25/25 | 18/20 | 15/15 | 9/10 | 5/5 | 92 | Promote. Interlocking blocks communicate component composition. |
| Manja | 24/25 | 25/25 | 18/20 | 14/15 | 9/10 | 5/5 | 95 | Promote. Specification sheet and path tree survive reduction. |
| Pajé | 22/25 | 25/25 | 18/20 | 15/15 | 9/10 | 5/5 | 94 | Promote. Five checkpoints sit on a durable rectilinear loop. |
| Xisnove | 23/25 | 25/25 | 18/20 | 14/15 | 9/10 | 5/5 | 94 | Promote. Protected rail block and live signal remain distinct. |

These are documented art-direction scores, not fabricated blind-review data.
The identity owner authorized canonical promotion before the five-person sample.
`review/blind-review.html` and `scripts/score-blind-reviews.py` remain the
post-publication recognition measurement and iteration mechanism.

## Promotion evidence

- Canonical marks, reverse marks, favicons, and logos are byte-identical to the
  selected v3 build products.
- Compact assets use only approved colors and contain no text, gradient,
  filter, mask, bitmap, or external reference.
- Every root SVG has a scalable `viewBox`, `role="img"`, `<title>`, `<desc>`,
  and `aria-labelledby` metadata; it has no fixed root width or height.
- Canonical wordmarks are HarfBuzz-shaped Instrument Sans SemiCondensed Bold
  outlines under the repository's SIL OFL notice.
- Chromium raster review covers paper, midnight reverse, 64/32/16 px favicon,
  outlined lockup, and unlabeled silhouettes.

## Artistic and product sources

- Fundação Athos Bulcão: combinatorial straight, semicircular, and diagonal
  geometric modules: <https://www.fundathos.org.br/noticia/304>
- MAM Rio: Hélio Oiticica's methodical geometric, color, and spatial
  experiments in *Metaesquemas*: <https://mam.rio/obras-de-arte/metaesquemas-1956-1958/>
- OpenAPI Initiative: `paths` as the endpoint and operation container:
  <https://learn.openapis.org/specification/paths.html>
