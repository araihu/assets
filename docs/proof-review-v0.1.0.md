# v0.1.0 identity proof review

Date: 2026-07-29

## Evidence boundary

`make proof` produced the static review artifact at `dist/proof/index.html`.
The local review URL was
`http://127.0.0.1:8766/index.html?product=araihu&mode=transparent&scheme=light`.

The document, `styles.css`, `app.js`, and every document-referenced local asset
returned HTTP 200 from a server rooted at `dist/proof`. JavaScript syntax checks
cleanly and the proof has no network dependency or `fetch` path.

Root in-app Browser evidence was available at the live `8766` URL: zero broken
images and zero console warnings or errors. The initial query state reported
`20/1114` evidence records; resetting controls to `all/all/all` reported
`1114/1114`. Sampled `plate` plus `dark` filtering reported `60` records and
matching scenario attributes.

Root's first viewport measurement found document overflow at 375 and 768 CSS
pixels (`documentElement.scrollWidth = 832`), while 1280 and 1440 matched their
viewport widths. The proof now constrains outer main, section, grid, rail, and
table contributions with `min-inline-size: 0` and `max-inline-size: 100%`, while
retaining internal family, exact-size, master, and table scrolling.

Root then isolated the remaining 375-pixel overflow to the license/provenance
section (`clientWidth = 343`, `scrollWidth = 817`): long visible source links
were unbreakable. License and provenance lists and links now use
`overflow-wrap: anywhere`; the post-fix browser regression is that document
scroll width equals each viewport at 375, 768, 1280, and 1440 pixels, while the
named internal rails may still scroll.

| Viewport | Document inner/scroll width | Window scrollX | Broken images | Family figures | Internal scrollers |
|---:|---:|---:|---:|---:|---:|
| 375 | 375 / 375 | 0 | 0 | 5 | 7 |
| 768 | 768 / 768 | 0 | 0 | 5 | 5 |
| 1280 | 1280 / 1280 | 0 | 0 | 5 | 5 |
| 1440 | 1440 / 1440 | 0 | 0 | 5 | 5 |

Final root Browser screenshots are retained at
`review/screenshots/v0.1-proof-{375,768,1280,1440}.png`.

| Screenshot | MIME | Dimensions | SHA-256 |
|---|---|---:|---|
| `v0.1-proof-375.png` | `image/png` | 375 x 900 | `5cb00d41139f4d97eed53e8ebe8f68f97cd536dd48f69e1852eef1cd1e5df28f` |
| `v0.1-proof-768.png` | `image/png` | 768 x 900 | `5392f7c370aa36218ec7f1e604bf9014d4c0b365fdd6403d5f50a8818dccc510` |
| `v0.1-proof-1280.png` | `image/png` | 1280 x 900 | `d634f0d7b1f266b84510a23294d7b50e805d9400396e21c56fe66e7c494ae538` |
| `v0.1-proof-1440.png` | `image/png` | 1440 x 900 | `5ae99fab52a42705e60847f64aa8ad86d530395ace083ab3dcde9427b9247f6a` |

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
- Fixed: outer proof containers no longer inherit the large min-content widths
  of literal specimens; only the intended comparison, exact-size, master, and
  geometry rails scroll horizontally.
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
