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

- `logos/araihu-logo.svg`, `logos/araihu-mark.svg`, and `logos/araihu-favicon.svg` form the Arai Hû organization identity.
- `logos/goshtoso-mark.svg` mirrors Goshtoso's canonical wave icon at source commit `e24d00b3f685f3c1a5a695f8c8a36f732e677317`; see `LICENSES/goshtoso-MIT.txt`.
- `logos/manja-logo.svg`, `logos/manja-mark.svg`, and `logos/manja-favicon.svg` are cleared Manja brand assets.
- Every product now has a full logo, compact mark, and favicon. Product marks retain the Arai Hû midnight, cobalt, and lime-horizon grammar.

Each SVG is source-controlled, self-contained, and suitable for web and documentation use. Do not add generated or third-party artwork without provenance and redistribution authority. Xisnove brand work is intentionally deferred while naming research is open.

## V2 design review

Recognizable, product-linked candidate marks live under `concepts/v2/` and do
not replace the released `logos/` assets. Open `review/logo-system-v2.html` for
the contact sheet and `review/blind-review.html` for the independent recognition
test. Validate the SVG system with `scripts/validate-logo-system.sh`; aggregate
five or more blind-review exports with `scripts/score-blind-reviews.py`.

## Goshtoso theme

`themes/araihu.css` is Arai Hû's organization theme for Goshtoso. It is not a Goshtoso base theme.

Load it after the Goshtoso stylesheet and set the product root to `data-theme="araihu"`. The theme provides daylight and `.dark` token pairs: paper and storm-blue in light mode; black-cloud navy, cobalt, and lime horizon in dark mode.

```html
<link rel="stylesheet" href="/assets/goshtoso.css">
<link rel="stylesheet" href="/assets/araihu-theme.css">
<html data-theme="araihu">
```
