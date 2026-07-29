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

Transparent variants use optically normalized viewBoxes: icons share a square
canvas with their dominant art dimension at 76%, while logos carry 8% padding
around their measured art bounds. Use these variants in controlled UI and site
surfaces. Background variants retain the full source canvas for visual review
and surfaces that need a supplied plate. Platform-ready launcher and store files
live under `dist/v11/`; those exports add the explicit safe-area, raster sizes,
layers, and metadata required by each target. The normalization changes framing
only, never path geometry.

For an external image, use the SVG directly. Its embedded
`prefers-color-scheme` fallback follows light and dark browser color schemes:

```html
<img
  src="/assets/araihu-logo-transparent.svg"
  alt="Arai Hû"
  width="1781"
  height="330"
  style="width: auto; height: 32px"
>
```

The numeric `width` and `height` preserve the SVG aspect ratio before it loads;
CSS may set the rendered size. For an inline SVG in a Goshtoso-based app, load
`themes/araihu.css` and keep
`data-theme="araihu"` plus the usual `.dark` class on `<html>`. The SVG inherits
`--araihu-logo-surface`, `--araihu-logo-ink`, and `--araihu-logo-signal`, so an
explicit app theme overrides the operating-system preference.

Regenerate and validate the family with:

```sh
python3 scripts/derive-logo-system-v11.py
python3 scripts/build-platform-assets-v11.py
./scripts/validate-logo-system-v11.sh
./scripts/validate-platform-assets-v11.sh
```

### V11 platform packages

- `dist/v11/web/<product>/` provides SVG and 16/32 px favicons, 192/512 px
  PWA `any` and `maskable` PNGs, a manifest icon fragment, and a 180 px Apple
  touch icon. Merge the fragment's `icons` array into the application's own web
  manifest; application name, start URL, scope, and display mode stay owned by
  the consuming app.
- `dist/v11/android/<product>/res/` is a drop-in Android resource tree. API 26
  gets adaptive background and foreground layers, API 33 adds a monochrome
  layer for themed icons, and density-specific legacy launcher fallbacks remain
  available.
- `dist/v11/apple/<product>/Assets.xcassets/` is an iOS/iPadOS asset catalog with
  opaque 1024 px light and dark masters plus a grayscale tinted variant.

Native and maskable exports fit their art inside the Android 66/108 safe square,
which is stricter than the PWA maskable safe circle. Do not substitute the
canonical transparent SVG for these packaged launcher resources.

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

Review v11 at exact web and mobile production sizes in
`review/logo-system-v11.html`. Serve the repository over HTTP so its in-browser
geometry audit can load sibling SVGs. Product, surface, and proof-scheme choices
remain shareable as query parameters; the page compares browser, navigation,
app UI, launcher, notification, tab-bar, and store contexts.
The measured problem, normalization decision, and adoption thresholds are
recorded in `review/v11-assessment.md`.

## Goshtoso theme

`themes/araihu.css` is Arai Hû's organization theme for Goshtoso. It is the
default for every Arai Hû-owned product, site, and demo, while remaining
separate from Goshtoso's base theme catalogue for external consumers.

Load it after the Goshtoso stylesheet and set the product root to `data-theme="araihu"`. The theme provides daylight and `.dark` token pairs: paper and storm-blue in light mode; black-cloud navy, cobalt, and lime horizon in dark mode.

```html
<link rel="stylesheet" href="/assets/goshtoso.css">
<link rel="stylesheet" href="/assets/araihu-theme.css">
<html data-theme="araihu">
```
