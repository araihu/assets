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
- The family uses a 128-unit constructive grid, rounded 14-unit strokes, and
  the Arai Hû midnight, cobalt, paper, and lime signal palette.
- Only Arai Hû uses the charged cloud. Product signs stand alone: composable
  interface panels for Goshtoso, an open API publication for Manja, a durable
  route for Pajé, and a three-state monitoring signal for Xisnove.

Each SVG is source-controlled, self-contained, scalable through `viewBox`, and
suitable for web and documentation use. Canonical wordmarks contain SVG paths,
not runtime text or a font dependency. Do not add generated or third-party
artwork without provenance and redistribution authority.

### Adaptive v11

The Recraft-derived v11 family lives under `concepts/v11/`. Each product has
an icon and horizontal logo, both with adaptive-background and transparent
variants. Shape geometry comes unchanged from the approved exports under
`recraft/`; the v11 build assigns every path one semantic color role:
`surface`, `ink`, or `signal`.

For an external image, use the SVG directly. Its embedded
`prefers-color-scheme` fallback follows light and dark browser color schemes:

```html
<img src="/assets/araihu-logo-transparent.svg" alt="Arai Hû">
```

For an inline SVG in a Goshtoso-based app, load `themes/araihu.css` and keep
`data-theme="araihu"` plus the usual `.dark` class on `<html>`. The SVG inherits
`--araihu-logo-surface`, `--araihu-logo-ink`, and `--araihu-logo-signal`, so an
explicit app theme overrides the operating-system preference.

Regenerate and validate the family with:

```sh
python3 scripts/derive-logo-system-v11.py
./scripts/validate-logo-system-v11.sh
```

## Design review

The promoted v10 construction sources live under `concepts/v10/`; rejected
directions remain under `concepts/v2/` through `concepts/v9/`. Compare every
ready version in `review/screenshots/logo-system-all-versions.png`, inspect v10
at normal, monochrome, and 16 px sizes in
`review/screenshots/logo-system-v10.png`, and inspect product contexts in
`review/screenshots/context/site-context-current-v10.png`. Validate canonical
assets with `scripts/validate-logo-system.sh`. Run the interactive v10 blind
review at `review/blind-review.html`; its printable A4 form is generated from
`review/blind-review-print.html` with `scripts/render-blind-review-pdf.sh`.
The final PDF is under `output/pdf/`; page proofs are retained as
`review/screenshots/blind-review-v10-page-1.png` and
`review/screenshots/blind-review-v10-page-2.png` for visual comparison.

## Goshtoso theme

`themes/araihu.css` is Arai Hû's organization theme for Goshtoso. It is not a Goshtoso base theme.

Load it after the Goshtoso stylesheet and set the product root to `data-theme="araihu"`. The theme provides daylight and `.dark` token pairs: paper and storm-blue in light mode; black-cloud navy, cobalt, and lime horizon in dark mode.

```html
<link rel="stylesheet" href="/assets/goshtoso.css">
<link rel="stylesheet" href="/assets/araihu-theme.css">
<html data-theme="araihu">
```
