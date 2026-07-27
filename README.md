# Arai Hû assets

Shared, release-ready brand assets for Arai Hû projects.

## License and use

Unless a more specific notice applies, this repository is licensed under the
[Apache License 2.0](LICENSE). Projects owned by the Arai Hû GitHub
organization are expressly authorized to use and redistribute these assets in
products, documentation, websites, and release artifacts.

Third-party material retains its original license under [LICENSES/](LICENSES/).
The Apache license does not grant trademark rights or permission to imply Arai
Hû endorsement or affiliation.

## Logos

- Every product has a full outlined logo, compact mark, reverse mark, and
  favicon under `logos/`.
- The family uses an 8 px constructive grid, open silhouettes, and the Arai Hû
  midnight, cobalt, paper, and lime signal palette.
- Meaning comes from each product's real work: storm front for Arai Hû,
  composable interface blocks for Goshtoso, an OpenAPI path document for Manja,
  a five-stage durable loop for Pajé, and a protected monitoring signal for
  Xisnove.

Each SVG is source-controlled, self-contained, scalable through `viewBox`, and
suitable for web and documentation use. Canonical wordmarks contain SVG paths,
not runtime text or a font dependency. Do not add generated or third-party
artwork without provenance and redistribution authority.

## Design review

The promoted v3 construction sources and rejected alternatives live under
`concepts/v3/`. Open `review/logo-system-v3.html` for the canonical contact
sheet, `review/logo-system-v3-exploration.html` for the A/B exploration, and
`review/blind-review.html` for the independent recognition test. Validate the
promoted system with `scripts/validate-logo-system-v3.sh`; aggregate five or
more blind-review exports with `scripts/score-blind-reviews.py`.

## Goshtoso theme

`themes/araihu.css` is Arai Hû's organization theme for Goshtoso. It is not a Goshtoso base theme.

Load it after the Goshtoso stylesheet and set the product root to `data-theme="araihu"`. The theme provides daylight and `.dark` token pairs: paper and storm-blue in light mode; black-cloud navy, cobalt, and lime horizon in dark mode.

```html
<link rel="stylesheet" href="/assets/goshtoso.css">
<link rel="stylesheet" href="/assets/araihu-theme.css">
<html data-theme="araihu">
```
