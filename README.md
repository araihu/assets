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

## Design review

The promoted v10 construction sources live under `concepts/v10/`; rejected
directions remain under `concepts/v2/` through `concepts/v9/`. Compare every
ready version in `review/screenshots/logo-system-all-versions.png`, inspect v10
at normal, monochrome, and 16 px sizes in
`review/screenshots/logo-system-v10.png`, and inspect product contexts in
`review/screenshots/context/site-context-current-v10.png`. Validate canonical
assets with `scripts/validate-logo-system.sh`.

## Goshtoso theme

`themes/araihu.css` is Arai Hû's organization theme for Goshtoso. It is not a Goshtoso base theme.

Load it after the Goshtoso stylesheet and set the product root to `data-theme="araihu"`. The theme provides daylight and `.dark` token pairs: paper and storm-blue in light mode; black-cloud navy, cobalt, and lime horizon in dark mode.

```html
<link rel="stylesheet" href="/assets/goshtoso.css">
<link rel="stylesheet" href="/assets/araihu-theme.css">
<html data-theme="araihu">
```
