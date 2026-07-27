# Arai Hû assets

Shared, release-ready brand assets for Arai Hû projects.

## Logos

- `logos/paje-mark.svg` preserves Pajé’s existing compact `P/` mark, blue field, and orange offset.

Each SVG is source-controlled, self-contained, and suitable for web and documentation use. Do not add generated or third-party artwork without provenance and redistribution authority. Xisnove brand work is intentionally deferred while naming research is open.

## Goshtoso theme

`themes/araihu.css` is Arai Hû's organization theme for Goshtoso. It is not a Goshtoso base theme.

Load it after the Goshtoso stylesheet and set the product root to `data-theme="araihu"`. The theme provides daylight and `.dark` token pairs: paper and storm-blue in light mode; black-cloud navy, cobalt, and lime horizon in dark mode.

```html
<link rel="stylesheet" href="/assets/goshtoso.css">
<link rel="stylesheet" href="/assets/araihu-theme.css">
<html data-theme="araihu">
```
