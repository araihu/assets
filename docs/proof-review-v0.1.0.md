# v0.1.0 identity proof review

Date: 2026-07-29

## Evidence boundary

`make proof` produced the static review artifact at `dist/proof/index.html`.
The local review URL was
`http://127.0.0.1:8766/index.html?product=araihu&mode=transparent&scheme=light`.

The document, `styles.css`, `app.js`, and every document-referenced local asset
returned HTTP 200 from a server rooted at `dist/proof`. JavaScript syntax checks
cleanly and the proof has no network dependency or `fetch` path.

The in-app Browser binding was unavailable in this review session. Therefore
the requested 375, 768, 1280, and 1440 CSS-pixel visual/keyboard/zoom passes,
console observation, and screenshots are not claimed. No screenshot was
retained: unavailable browser rendering is not review evidence.

## Findings and disposition

### Asset geometry

- No canonical SVG geometry changed. Optical balance and small-size silhouettes
  still require a browser-backed reviewer at 375, 768, 1280, and 1440 pixels.
- Circle and squircle samples now apply real clipping to the catalog launcher
  artifact. This is proof evidence only, not a new canonical asset.

### Recipe and framing

- Transparent artwork now appears separately on checker, paper, and midnight
  stress backgrounds. These reviewer backgrounds do not alter a catalog source
  plate.
- Plate samples preserve the source plate as published. The detail filters use
  catalog surface and appearance metadata rather than recoloring artwork.

### Proof layout

- Fixed: the proof had no generated `index.html`; `make proof` now creates a
  deterministic self-contained page with local CSS, JavaScript, and copied
  evidence assets.
- Fixed: product/detail filters now have an all-products reset, visible evidence
  count, catalog surface filter, and catalog artwork-scheme filter. The
  five-product launcher baseline remains visible during drill-down.
- Fixed: literal 512 specimens stay inside an internal horizontal rail; detail
  grids do not collapse until below 736 CSS pixels; metadata uses `#40566b` on
  paper, exceeding 4.5:1 contrast.
- Fixed: geometry evidence has a caption and scoped column headers. Web and
  mobile scenes now include enough chrome to inspect padding and collision.
- Impeccable detector found one proof-layout side-tab accent, which was changed
  to a top signal rule in this single proof-owned fix pass. Its Instrument Sans
  warning is a false positive: `DESIGN.md` pins Instrument Sans for this bench.

## Goshtoso parity and limits

This proof intentionally uses only static HTML, CSS, and dependency-free local
JavaScript. It does not import Goshtoso components, its asset pipeline, or its
interaction conventions; therefore it is not a Goshtoso visual-parity claim.
The reusable parity evidence is limited to semantic HTML, native keyboard
buttons, visible focus, reduced-motion handling, and locally served assets.
Consumer integration still needs a Goshtoso-owned browser pass after mounting
the generated assets in an application context.
